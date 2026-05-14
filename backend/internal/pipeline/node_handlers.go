// Package pipeline 内的节点处理器集合 — 这些 handler 在同一进程内被 Runner 调用,
// 不走 asynq 队列。每个 handler 接收 NodeContext{ Input, Params, Publisher, ... },
// 返回 output map 供下游节点取用。
//
// 设计要点:
//   - 节点 handler 不依赖 HTTP/asynq:直接调用 service / repo / adapter
//   - input 字段约定:
//     script_id, episode_id, storyboard_id, image_id, short_video_id, full_video_id
//     prompt, model_id, style_id 等
//   - 节点输出统一以同名字段写回,便于 edge.mapping 透传
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/adapter"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/storage"
)

// DefaultDeps 节点处理器需要的依赖(由 worker/main.go 在启动时注入)
type DefaultDeps struct {
	Repos      *repo.Repositories
	GetAdapter func(ctx context.Context, modelID int64) (adapter.Adapter, *model.Model, error)
	Store      storage.Storage
}

// RegisterDefaultNodeHandlers 把全部内置节点 handler 注册到 NodeHandlerRegistry
func RegisterDefaultNodeHandlers(reg *NodeHandlerRegistry, deps *DefaultDeps) {
	reg.Register(NodePromptGenerate, newPromptGenerate(deps))
	reg.Register(NodeStoryboardGenerate, newStoryboardGenerate(deps))
	reg.Register(NodeImageGenerate, newImageGenerate(deps))
	reg.Register(NodeVideoGenerate, newVideoGenerate(deps))
	reg.Register(NodeAudioTTS, newAudioTTS(deps))
	reg.Register(NodeStyleApply, newStyleApply(deps))
	reg.Register(NodeVideoCompose, newVideoCompose(deps))
	reg.Register(NodeScriptSplit, newScriptSplit(deps))
	reg.Register(NodeImageUpload, newImageUpload(deps))
	reg.Register(NodeReviewSubmit, newReviewSubmit(deps))
	reg.Register(NodeHumanApprove, newHumanApprove(deps))
}

// helpers

func intFromInput(m map[string]any, key string) int64 {
	if v, ok := m[key]; ok {
		switch x := v.(type) {
		case int64:
			return x
		case int:
			return int64(x)
		case float64:
			return int64(x)
		case string:
			// 数字字符串
			var n int64
			_, _ = fmt.Sscan(x, &n)
			return n
		}
	}
	return 0
}

func strFromInput(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// newPassthrough 直接把 input 原样返回为 output(占位 / 调试用)
func newPassthrough(name string) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		nc.Publisher(0.5, name+" passthrough")
		return nc.Input, nil
	}
}

// newScriptSplit 读取剧本,调用 LLM text adapter 拆分为分集,创建 episode 记录。
func newScriptSplit(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		scriptID := intFromInput(nc.Input, "script_id")
		if scriptID == 0 {
			return nil, errors.New("script.split: missing script_id")
		}
		modelID := nc.ModelID
		if modelID == 0 {
			modelID = intFromInput(nc.Input, "model_id")
		}
		if modelID == 0 {
			return nil, errors.New("script.split: missing model_id")
		}

		nc.Publisher(0.05, "fetch script")
		sc, err := d.Repos.Script.Get(nc.Ctx, scriptID)
		if err != nil {
			return nil, fmt.Errorf("script.split: load script %d: %w", scriptID, err)
		}

		ad, m, err := d.GetAdapter(nc.Ctx, modelID)
		if err != nil {
			return nil, fmt.Errorf("script.split: get adapter: %w", err)
		}
		if ad.Type() != adapter.TypeText {
			return nil, errors.New("script.split requires text model")
		}

		modelCode := ""
		if m != nil {
			modelCode = m.Code
		}
		nc.Publisher(0.2, "calling LLM "+modelCode)

		count := 12
		if v, ok := nc.Params["episode_count"]; ok {
			switch x := v.(type) {
			case int:
				count = x
			case float64:
				count = int(x)
			}
		}
		targetChars := 800
		if v, ok := nc.Params["target_chars"]; ok {
			switch x := v.(type) {
			case int:
				targetChars = x
			case float64:
				targetChars = int(x)
			}
		}

		prompt := buildSplitPrompt(sc.RawText, count, targetChars)
		req := &adapter.Request{
			Prompt: prompt,
			Params: map[string]any{
				"system":      "你是一个短视频剧本拆解专家,严格按用户要求输出 JSON。",
				"temperature": 0.4,
			},
		}
		for k, v := range nc.Params {
			req.Params[k] = v
		}
		resp, err := ad.Generate(nc.Ctx, req)
		if err != nil {
			return nil, fmt.Errorf("script.split: LLM call failed: %w", err)
		}
		if len(resp.Texts) == 0 {
			return nil, errors.New("script.split: empty LLM response")
		}

		nc.Publisher(0.7, "parsing episodes")
		eps, err := parseEpisodes(resp.Texts[0])
		if err != nil {
			return nil, fmt.Errorf("script.split: parse episodes: %w", err)
		}
		if len(eps) == 0 {
			return nil, errors.New("script.split: no episodes parsed")
		}
		for i := range eps {
			eps[i].ScriptID = scriptID
			if eps[i].EpNo == 0 {
				eps[i].EpNo = i + 1
			}
		}

		nc.Publisher(0.85, "saving episodes")
		_ = d.Repos.Episode.DeleteByScript(nc.Ctx, scriptID)
		if err := d.Repos.Episode.BulkCreate(nc.Ctx, eps); err != nil {
			return nil, fmt.Errorf("script.split: save episodes: %w", err)
		}

		nc.Publisher(0.95, "update script status")
		_ = d.Repos.Script.UpdateStatus(nc.Ctx, scriptID, 3) // 3 = split ok

		ids := make([]int64, len(eps))
		for i, ep := range eps {
			ids[i] = ep.ID
		}
		return map[string]any{
			"script_id":     scriptID,
			"episode_ids":   ids,
			"episode_count": len(eps),
		}, nil
	}
}

// buildSplitPrompt 构造 LLM 输入
func buildSplitPrompt(text string, count, targetChars int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请把下面这部剧本切分为大约 %d 集短视频,每集 %d-%d 字。\n", count, targetChars, targetChars+200))
	sb.WriteString("严格输出 JSON 数组,每个元素为 {\"ep_no\":int, \"title\":string, \"summary\":string, \"raw_segment\":string}。\n")
	sb.WriteString("不要输出额外的解释文本,只输出可被 json.Unmarshal 解析的数组。\n\n剧本内容:\n")
	sb.WriteString(text)
	return sb.String()
}

// parseEpisodes 容错地从 LLM 输出中提取 JSON 数组
func parseEpisodes(out string) ([]model.Episode, error) {
	s := stripJSONFence(out)
	// 找到第一个 '[' 和最后一个 ']'
	l := strings.Index(s, "[")
	r := strings.LastIndex(s, "]")
	if l < 0 || r <= l {
		return nil, errors.New("no JSON array found")
	}
	raw := s[l : r+1]
	var arr []struct {
		EpNo       int    `json:"ep_no"`
		Title      string `json:"title"`
		Summary    string `json:"summary"`
		RawSegment string `json:"raw_segment"`
	}
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil, err
	}
	res := make([]model.Episode, 0, len(arr))
	for _, e := range arr {
		res = append(res, model.Episode{
			EpNo:       e.EpNo,
			Title:      e.Title,
			Summary:    e.Summary,
			RawSegment: e.RawSegment,
			Status:     1,
		})
	}
	return res, nil
}

var fenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")

func stripJSONFence(s string) string {
	if m := fenceRe.FindStringSubmatch(s); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(s)
}

// newImageUpload 读取 image 记录,下载图片并上传到对象存储,更新 URL 与状态。
func newImageUpload(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		imageID := intFromInput(nc.Input, "image_id")
		if imageID == 0 {
			return nil, errors.New("image.upload: missing image_id")
		}
		if d.Store == nil {
			return nil, errors.New("image.upload: storage not configured")
		}

		nc.Publisher(0.1, "load image record")
		img, err := d.Repos.Image.Get(nc.Ctx, imageID)
		if err != nil {
			return nil, fmt.Errorf("image.upload: load image %d: %w", imageID, err)
		}
		if img.URL == "" {
			return nil, errors.New("image.upload: image has no URL")
		}

		nc.Publisher(0.3, "download image")
		cctx, cancel := context.WithTimeout(nc.Ctx, 60*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(cctx, http.MethodGet, img.URL, nil)
		if err != nil {
			return nil, fmt.Errorf("image.upload: build request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("image.upload: download failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("image.upload: bad status %d", resp.StatusCode)
		}

		nc.Publisher(0.6, "upload to storage")
		ext := ".jpg"
		ct := resp.Header.Get("Content-Type")
		switch {
		case strings.Contains(ct, "png"):
			ext = ".png"
		case strings.Contains(ct, "webp"):
			ext = ".webp"
		case strings.Contains(ct, "jpeg"):
			ext = ".jpg"
		}
		key := fmt.Sprintf("images/%d%s", img.ID, ext)
		publicURL, err := d.Store.Put(cctx, key, resp.Body, resp.ContentLength, ct)
		if err != nil {
			return nil, fmt.Errorf("image.upload: storage put: %w", err)
		}

		nc.Publisher(0.9, "update image record")
		img.URL = publicURL
		img.Status = 2 // succeeded
		if err := d.Repos.Image.Update(nc.Ctx, img); err != nil {
			return nil, fmt.Errorf("image.upload: update record: %w", err)
		}

		return map[string]any{
			"image_id":  img.ID,
			"image_url": img.URL,
		}, nil
	}
}

// newReviewSubmit 为 full_video 创建审核记录。
func newReviewSubmit(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		fullVideoID := intFromInput(nc.Input, "full_video_id")
		if fullVideoID == 0 {
			return nil, errors.New("review.submit: missing full_video_id")
		}

		nc.Publisher(0.1, "load full video")
		fv, err := d.Repos.Full.Get(nc.Ctx, fullVideoID)
		if err != nil {
			return nil, fmt.Errorf("review.submit: load full_video %d: %w", fullVideoID, err)
		}

		nc.Publisher(0.3, "find default review flow")
		flow, err := d.Repos.Review.DefaultFlow(nc.Ctx, "full_video")
		if err != nil {
			return nil, fmt.Errorf("review.submit: no default review flow: %w", err)
		}

		nc.Publisher(0.6, "create review record")
		rec := &model.ReviewRecord{
			TargetType:  "full_video",
			TargetID:    fv.ID,
			FlowID:      flow.ID,
			CurrentStep: 1,
			Status:      "pending",
		}
		if err := d.Repos.Review.CreateRecord(nc.Ctx, rec); err != nil {
			return nil, fmt.Errorf("review.submit: create record: %w", err)
		}

		nc.Publisher(0.9, "update full video status")
		_ = d.Repos.Full.UpdateStatus(nc.Ctx, fv.ID, "reviewing", 0, "")

		return map[string]any{
			"full_video_id":    fv.ID,
			"review_record_id": rec.ID,
			"flow_id":          flow.ID,
		}, nil
	}
}

// newHumanApprove 检查 full_video 的审核记录状态,未通过时返回错误阻断流水线。
func newHumanApprove(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		fullVideoID := intFromInput(nc.Input, "full_video_id")
		if fullVideoID == 0 {
			return nil, errors.New("human.approve: missing full_video_id")
		}

		nc.Publisher(0.2, "check review status")
		rec, err := d.Repos.Review.GetActiveRecord(nc.Ctx, "full_video", fullVideoID)
		if err != nil {
			// 没有 pending 记录时尝试找任意记录
			var allRecs []model.ReviewRecord
			if derr := d.Repos.DB.WithContext(nc.Ctx).
				Where("target_type = ? AND target_id = ?", "full_video", fullVideoID).
				Order("id desc").Limit(1).Find(&allRecs).Error; derr == nil && len(allRecs) > 0 {
				rec = &allRecs[0]
			} else {
				return nil, fmt.Errorf("human.approve: no review record for full_video %d", fullVideoID)
			}
		}

		nc.Publisher(0.5, fmt.Sprintf("review status: %s", rec.Status))
		if rec.Status != "approved" {
			return nil, fmt.Errorf("human.approve: review not approved (status=%s, record_id=%d)", rec.Status, rec.ID)
		}

		nc.Publisher(0.9, "approved")
		return map[string]any{
			"full_video_id":    fullVideoID,
			"review_record_id": rec.ID,
			"approved":         true,
		}, nil
	}
}

// newPromptGenerate 调用 LLM 为 episode 生成 prompt JSON,落库为 episode_prompts.current。
func newPromptGenerate(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		episodeID := intFromInput(nc.Input, "episode_id")
		if episodeID == 0 {
			return nil, errors.New("prompt.generate: missing episode_id")
		}
		modelID := nc.ModelID
		if modelID == 0 {
			modelID = intFromInput(nc.Input, "model_id")
		}
		if modelID == 0 {
			return nil, errors.New("prompt.generate: missing model_id")
		}
		nc.Publisher(0.1, "fetch episode")
		ep, err := d.Repos.Episode.Get(nc.Ctx, episodeID)
		if err != nil {
			return nil, err
		}
		ad, _, err := d.GetAdapter(nc.Ctx, modelID)
		if err != nil {
			return nil, err
		}
		if ad.Type() != adapter.TypeText {
			return nil, errors.New("prompt.generate requires text model")
		}
		nc.Publisher(0.3, "calling LLM")
		req := &adapter.Request{
			Prompt: fmt.Sprintf(
				"基于下面这一集剧本输出 JSON {summary,shots[{shot_no,shot_type,camera,scene,characters,action,dialogue,duration_sec,image_prompt,video_prompt}]}\n第%d集 %s\n摘要: %s\n剧本:\n%s",
				ep.EpNo, ep.Title, ep.Summary, ep.RawSegment),
			Params: map[string]any{
				"system":      "你是分镜与提示词专家,严格输出 JSON,不要解释。",
				"temperature": 0.7,
			},
		}
		for k, v := range nc.Params {
			req.Params[k] = v
		}
		resp, err := ad.Generate(nc.Ctx, req)
		if err != nil {
			return nil, err
		}
		if len(resp.Texts) == 0 {
			return nil, errors.New("empty LLM response")
		}
		// 落库
		raw := []byte(resp.Texts[0])
		nc.Publisher(0.9, "save prompt")
		row := &model.EpisodePrompt{
			EpisodeID: episodeID,
			Content:   model.JSON(raw),
			ModelID:   modelID,
			Status:    1,
		}
		if err := d.Repos.Prompt.CreateAsCurrent(nc.Ctx, row); err != nil {
			return nil, err
		}
		return map[string]any{
			"episode_id": episodeID,
			"prompt_id":  row.ID,
		}, nil
	}
}

// newStoryboardGenerate 基于当前 prompt JSON 的 shots[] 落库 storyboards
func newStoryboardGenerate(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		episodeID := intFromInput(nc.Input, "episode_id")
		if episodeID == 0 {
			return nil, errors.New("storyboard.generate: missing episode_id")
		}
		nc.Publisher(0.1, "fetch current prompt")
		cur, err := d.Repos.Prompt.GetCurrent(nc.Ctx, episodeID)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Shots []struct {
				ShotNo      int      `json:"shot_no"`
				ShotType    string   `json:"shot_type"`
				Camera      string   `json:"camera"`
				Scene       string   `json:"scene"`
				Characters  []string `json:"characters"`
				Action      string   `json:"action"`
				Dialogue    string   `json:"dialogue"`
				DurationSec int      `json:"duration_sec"`
			} `json:"shots"`
		}
		if err := json.Unmarshal(cur.Content, &doc); err != nil {
			return nil, fmt.Errorf("invalid prompt json: %w", err)
		}
		if len(doc.Shots) == 0 {
			return nil, errors.New("no shots in prompt")
		}
		nc.Publisher(0.5, fmt.Sprintf("parsed %d shots", len(doc.Shots)))
		list := make([]model.Storyboard, 0, len(doc.Shots))
		for _, sh := range doc.Shots {
			chs, _ := json.Marshal(sh.Characters)
			st := "medium"
			if sh.ShotType != "" {
				st = sh.ShotType
			}
			cm := "static"
			if sh.Camera != "" {
				cm = sh.Camera
			}
			dur := sh.DurationSec
			if dur <= 0 {
				dur = 15
			}
			list = append(list, model.Storyboard{
				EpisodeID:    episodeID,
				PromptID:     cur.ID,
				ShotNo:       sh.ShotNo,
				ShotType:     st,
				CameraMotion: cm,
				SceneDesc:    sh.Scene,
				Characters:   model.JSON(chs),
				Action:       sh.Action,
				Dialogue:     sh.Dialogue,
				DurationSec:  dur,
				Status:       1,
			})
		}
		if err := d.Repos.Story.DeleteByEpisode(nc.Ctx, episodeID); err != nil {
			return nil, err
		}
		if err := d.Repos.Story.BulkCreate(nc.Ctx, list); err != nil {
			return nil, err
		}
		ids := make([]int64, len(list))
		for i, sb := range list {
			ids[i] = sb.ID
		}
		return map[string]any{
			"episode_id":     episodeID,
			"storyboard_ids": ids,
		}, nil
	}
}

// newImageGenerate 为单个 storyboard 调用 image 模型,落库 image。
func newImageGenerate(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		sbID := intFromInput(nc.Input, "storyboard_id")
		if sbID == 0 {
			return nil, errors.New("image.generate: missing storyboard_id")
		}
		modelID := nc.ModelID
		if modelID == 0 {
			modelID = intFromInput(nc.Input, "model_id")
		}
		if modelID == 0 {
			return nil, errors.New("image.generate: missing model_id")
		}
		nc.Publisher(0.1, "load storyboard")
		sb, err := d.Repos.Story.Get(nc.Ctx, sbID)
		if err != nil {
			return nil, err
		}
		ad, _, err := d.GetAdapter(nc.Ctx, modelID)
		if err != nil {
			return nil, err
		}
		if ad.Type() != adapter.TypeImage {
			return nil, errors.New("image.generate requires image model")
		}
		prompt := strFromInput(nc.Input, "image_prompt")
		if prompt == "" {
			prompt = sb.SceneDesc
		}
		nc.Publisher(0.3, "call image model")
		resp, err := ad.Generate(nc.Ctx, &adapter.Request{
			Prompt: prompt,
			Params: nc.Params,
		})
		if err != nil {
			return nil, err
		}
		if len(resp.ImageURLs) == 0 {
			return nil, errors.New("image model returned no URL")
		}
		nc.Publisher(0.9, "save image")
		img := &model.Image{
			StoryboardID: sbID,
			SrcType:      "generated",
			URL:          resp.ImageURLs[0],
			Prompt:       prompt,
			ModelID:      modelID,
			Status:       2,
		}
		if err := d.Repos.Image.Create(nc.Ctx, img); err != nil {
			return nil, err
		}
		return map[string]any{
			"storyboard_id": sbID,
			"image_id":      img.ID,
			"image_url":     img.URL,
		}, nil
	}
}

// newVideoGenerate 为单个 storyboard 调用 video 模型,落库 short_video。
func newVideoGenerate(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		sbID := intFromInput(nc.Input, "storyboard_id")
		if sbID == 0 {
			return nil, errors.New("video.generate: missing storyboard_id")
		}
		modelID := nc.ModelID
		if modelID == 0 {
			modelID = intFromInput(nc.Input, "model_id")
		}
		if modelID == 0 {
			return nil, errors.New("video.generate: missing model_id")
		}
		nc.Publisher(0.1, "load storyboard")
		sb, err := d.Repos.Story.Get(nc.Ctx, sbID)
		if err != nil {
			return nil, err
		}
		ad, _, err := d.GetAdapter(nc.Ctx, modelID)
		if err != nil {
			return nil, err
		}
		if ad.Type() != adapter.TypeVideo {
			return nil, errors.New("video.generate requires video model")
		}
		// 创建占位记录
		sv := &model.ShortVideo{
			StoryboardID: sbID,
			SrcType:      "generated",
			Prompt:       sb.SceneDesc,
			ModelID:      modelID,
			Status:       "running",
		}
		if err := d.Repos.Short.Create(nc.Ctx, sv); err != nil {
			return nil, err
		}

		inputs := []string{}
		if v, ok := nc.Input["image_url"].(string); ok && v != "" {
			inputs = append(inputs, v)
		}
		nc.Publisher(0.3, "call video model")
		resp, err := ad.Generate(nc.Ctx, &adapter.Request{
			Prompt: sb.SceneDesc,
			Inputs: inputs,
			Params: nc.Params,
		})
		if err != nil {
			_ = d.Repos.Short.UpdateStatus(nc.Ctx, sv.ID, "failed", err.Error())
			return nil, err
		}
		if len(resp.VideoURLs) == 0 {
			_ = d.Repos.Short.UpdateStatus(nc.Ctx, sv.ID, "failed", "no video url")
			return nil, errors.New("no video url")
		}
		nc.Publisher(0.9, "save short video")
		sv.VideoURL = resp.VideoURLs[0]
		sv.DurationMs = resp.DurationMs
		sv.Status = "succeeded"
		_ = d.Repos.Short.Update(nc.Ctx, sv)
		return map[string]any{
			"storyboard_id":  sbID,
			"short_video_id": sv.ID,
			"video_url":      sv.VideoURL,
			"duration_ms":    sv.DurationMs,
		}, nil
	}
}

// newAudioTTS 调用 TTS 模型,音频字节回写到 short_video.audio_url(若 storyboard_id 提供),
// 否则只返回 audio_bytes 给下游。
func newAudioTTS(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		text := strFromInput(nc.Input, "tts_text")
		if text == "" {
			text = strFromInput(nc.Input, "dialogue")
		}
		if text == "" {
			return nil, errors.New("audio.tts: missing tts_text or dialogue")
		}
		modelID := nc.ModelID
		if modelID == 0 {
			modelID = intFromInput(nc.Input, "model_id")
		}
		if modelID == 0 {
			return nil, errors.New("audio.tts: missing model_id")
		}
		ad, _, err := d.GetAdapter(nc.Ctx, modelID)
		if err != nil {
			return nil, err
		}
		if ad.Type() != adapter.TypeAudio {
			return nil, errors.New("audio.tts requires audio model")
		}
		nc.Publisher(0.3, "call tts model")
		resp, err := ad.Generate(nc.Ctx, &adapter.Request{Prompt: text})
		if err != nil {
			return nil, err
		}
		out := map[string]any{
			"tts_text":    text,
			"duration_ms": resp.DurationMs,
		}
		if resp.Raw != nil {
			if b, ok := resp.Raw["audio_bytes"].([]byte); ok {
				out["audio_bytes_len"] = len(b)
				// 暂不上传 OSS,留待 video.compose 节点统一合成
				out["audio_bytes_b64"] = bytesPlaceholder(b)
			}
		}
		return out, nil
	}
}

// newStyleApply 把风格挂到 storyboard
func newStyleApply(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		sbID := intFromInput(nc.Input, "storyboard_id")
		styleID := intFromInput(nc.Input, "style_id")
		if sbID == 0 || styleID == 0 {
			return nil, errors.New("style.apply: missing storyboard_id or style_id")
		}
		nc.Publisher(0.5, "apply style")
		if err := d.Repos.Story.ApplyStyle(nc.Ctx, sbID, styleID, 0); err != nil {
			return nil, err
		}
		return map[string]any{
			"storyboard_id": sbID,
			"style_id":      styleID,
		}, nil
	}
}

// newVideoCompose 入队 video.compose,但这里直接调用 FullVideo Repos 也行;
// 为简化,DAG 内的 video.compose 节点只生成一个 full_video 草稿并入队渲染。
func newVideoCompose(d *DefaultDeps) NodeHandler {
	return func(nc *NodeContext) (map[string]any, error) {
		projectID := intFromInput(nc.Input, "project_id")
		// 上游应传入 short_video_ids 列表
		var ids []int64
		if raw, ok := nc.Input["short_video_ids"].([]any); ok {
			for _, v := range raw {
				if n, ok2 := v.(float64); ok2 {
					ids = append(ids, int64(n))
				}
			}
		} else if raw2, ok := nc.Input["short_video_ids"].([]int64); ok {
			ids = raw2
		}
		if len(ids) == 0 {
			return nil, errors.New("video.compose: missing short_video_ids")
		}
		clips := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			clips = append(clips, map[string]any{"short_video_id": id})
		}
		tl := map[string]any{
			"clips": clips,
		}
		tlBytes, _ := json.Marshal(tl)
		full := &model.FullVideo{
			ProjectID: projectID,
			Name:      fmt.Sprintf("auto-%d", time.Now().Unix()),
			Timeline:  model.JSON(tlBytes),
			Status:    "queued",
		}
		if err := d.Repos.Full.Create(nc.Ctx, full); err != nil {
			return nil, err
		}
		nc.Publisher(0.5, fmt.Sprintf("full_video %d created (compose 需手动 render 或追加 worker)", full.ID))
		return map[string]any{
			"full_video_id": full.ID,
			"project_id":    projectID,
		}, nil
	}
}

func bytesPlaceholder(b []byte) string {
	// 仅作为节点输出标记,不做 base64 实际编码,避免膨胀。这里返回长度提示。
	return fmt.Sprintf("<%d bytes>", len(b))
}

// persistRemoteAsset 把 adapter 返回的外链下载并落到对象存储,返回稳定的内部 URL。
// store 为 nil 或下载失败时退化到原 URL,不阻塞主流程。
func persistRemoteAsset(ctx context.Context, store storage.Storage, namespace, srcURL string) string {
	if store == nil || srcURL == "" {
		return srcURL
	}
	// 已经是本地 /uploads 相对路径(同一 store 二次落盘没意义)
	if strings.HasPrefix(srcURL, "/uploads/") {
		return srcURL
	}
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return srcURL
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return srcURL
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return srcURL
	}
	ext := ".bin"
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "image/jpeg"):
		ext = ".jpg"
	case strings.HasPrefix(ct, "image/png"):
		ext = ".png"
	case strings.HasPrefix(ct, "image/webp"):
		ext = ".webp"
	case strings.HasPrefix(ct, "video/mp4"):
		ext = ".mp4"
	case strings.HasPrefix(ct, "video/webm"):
		ext = ".webm"
	}
	key := fmt.Sprintf("%s/%d%s", namespace, time.Now().UnixNano(), ext)
	url, err := store.Put(cctx, key, resp.Body, resp.ContentLength, ct)
	if err != nil {
		return srcURL
	}
	return url
}

// ioReadAll 兼容 Go 1.15+ 的 io.ReadAll 包装(避免旧版本无此函数)。
// 实际上 Go 1.16+ 已有 io.ReadAll,这里保留作为兜底。
func ioReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
