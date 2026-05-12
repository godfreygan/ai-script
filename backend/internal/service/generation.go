package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/adapter"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/queue"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/storage"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/ws"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// persistRemoteAsset 把 adapter 返回的外链(litellm/上游 CDN, 通常有效期短)
// 下载并落到对象存储,返回稳定的内部 URL。store 为 nil 或下载失败时退化到原 URL,
// 不阻塞主流程 — 这是加固层。
func persistRemoteAsset(ctx context.Context, store storage.Storage, namespace, srcURL string, log *zap.Logger) string {
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
		if log != nil {
			log.Warn("persistRemoteAsset: build request", zap.String("url", srcURL), zap.Error(err))
		}
		return srcURL
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if log != nil {
			log.Warn("persistRemoteAsset: download", zap.String("url", srcURL), zap.Error(err))
		}
		return srcURL
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if log != nil {
			log.Warn("persistRemoteAsset: bad status", zap.String("url", srcURL), zap.Int("code", resp.StatusCode))
		}
		return srcURL
	}
	ext := strings.ToLower(path.Ext(srcURL))
	if i := strings.IndexAny(ext, "?#"); i >= 0 {
		ext = ext[:i]
	}
	if ext == "" {
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
		default:
			ext = ".bin"
		}
	}
	rnd := make([]byte, 8)
	_, _ = rand.Read(rnd)
	now := time.Now()
	key := fmt.Sprintf("%s/%04d/%02d/%02d/%s%s",
		namespace, now.Year(), now.Month(), now.Day(), hex.EncodeToString(rnd), ext)
	url, err := store.Put(cctx, key, resp.Body, resp.ContentLength, resp.Header.Get("Content-Type"))
	if err != nil {
		if log != nil {
			log.Warn("persistRemoteAsset: storage put", zap.String("key", key), zap.Error(err))
		}
		return srcURL
	}
	return url
}

// ============== Script ==============

type ScriptService struct {
	r   *repo.Repositories
	tc  *queue.Client
	hub *ws.Hub
	log *zap.Logger
}

type CreateScriptInput struct {
	ProjectID int64  `json:"project_id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	RawText   string `json:"raw_text" binding:"required"`
	SourceURL string `json:"source_url"`
}

type SplitScriptInput struct {
	ModelID     int64          `json:"model_id" binding:"required"`
	EpisodeCnt  int            `json:"episode_count"`
	TargetChars int            `json:"target_chars"`
	Params      map[string]any `json:"params"`
}

type splitPayload struct {
	ScriptID    int64          `json:"script_id"`
	ModelID     int64          `json:"model_id"`
	EpisodeCnt  int            `json:"episode_count"`
	TargetChars int            `json:"target_chars"`
	Params      map[string]any `json:"params"`
	UserID      int64          `json:"user_id"`
}

func (s *ScriptService) List(ctx context.Context, q *repo.ListScriptsQuery) ([]model.Script, int64, error) {
	return s.r.Script.List(ctx, q)
}

func (s *ScriptService) Get(ctx context.Context, id int64) (*model.Script, error) {
	sc, err := s.r.Script.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return sc, nil
}

func (s *ScriptService) Create(ctx context.Context, in *CreateScriptInput, uid int64) (*model.Script, error) {
	sc := &model.Script{
		ProjectID:      in.ProjectID,
		Name:           in.Name,
		SourceURL:      in.SourceURL,
		RawText:        in.RawText,
		CurrentVersion: 1,
		Status:         1, // uploaded
	}
	sc.CreatedBy = uid
	sc.UpdatedBy = uid
	if err := s.r.Script.Create(ctx, sc); err != nil {
		return nil, err
	}
	// 同步写入 v1 版本
	_ = s.r.Script.AddVersion(ctx, &model.ScriptVersion{
		ScriptID:  sc.ID,
		VersionNo: 1,
		Content:   in.RawText,
		CommitMsg: "initial",
		CreatedBy: uid,
	})
	return sc, nil
}

func (s *ScriptService) Delete(ctx context.Context, id int64) error {
	if err := s.r.Episode.DeleteByScript(ctx, id); err != nil {
		return err
	}
	return s.r.Script.Delete(ctx, id)
}

func (s *ScriptService) ListEpisodes(ctx context.Context, scriptID int64) ([]model.Episode, error) {
	return s.r.Episode.ListByScript(ctx, scriptID)
}

// Split 异步分集 - 入队后立即返回任务 ID,worker 端调用 LLM 并写入 episodes
func (s *ScriptService) Split(ctx context.Context, scriptID int64, in *SplitScriptInput, uid int64) (string, error) {
	if _, err := s.r.Script.Get(ctx, scriptID); err != nil {
		return "", errcode.ErrNotFound
	}
	payload, _ := json.Marshal(splitPayload{
		ScriptID:    scriptID,
		ModelID:     in.ModelID,
		EpisodeCnt:  in.EpisodeCnt,
		TargetChars: in.TargetChars,
		Params:      in.Params,
		UserID:      uid,
	})
	return s.tc.Enqueue(ctx, TaskScriptSplit, payload, asynq.Queue("default"), asynq.MaxRetry(3))
}

// HandleSplitTask 是 worker 端的 script.split 处理器
func (s *ScriptService) HandleSplitTask(modelSvc *ModelService) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p splitPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		topic := fmt.Sprintf("script:%d", p.ScriptID)
		// markFailed 在任意失败分支前把 script.status 置 4(failed),让前端能判定
		markFailed := func(reason string) {
			if err := s.r.Script.UpdateStatus(ctx, p.ScriptID, 4); err != nil {
				s.log.Warn("mark script failed", zap.Int64("script_id", p.ScriptID), zap.Error(err))
			}
			s.publish(topic, "error", 0, reason)
		}
		s.publish(topic, "progress", 0.05, "fetching script")

		sc, err := s.r.Script.Get(ctx, p.ScriptID)
		if err != nil {
			s.publish(topic, "error", 0, "script not found")
			return err
		}
		ad, m, err := modelSvc.GetAdapter(ctx, p.ModelID)
		if err != nil {
			markFailed("model adapter unavailable: " + err.Error())
			return err
		}
		if ad.Type() != adapter.TypeText {
			markFailed("model is not a text model")
			return errors.New("script split requires a text model")
		}

		s.publish(topic, "progress", 0.2, "calling LLM "+m.Code)

		count := p.EpisodeCnt
		if count <= 0 {
			count = 12
		}
		prompt := buildSplitPrompt(sc.RawText, count, p.TargetChars)
		req := &adapter.Request{
			Prompt: prompt,
			Params: map[string]any{
				"system":      "你是一个短视频剧本拆解专家,严格按用户要求输出 JSON。",
				"temperature": 0.4,
			},
		}
		for k, v := range p.Params {
			req.Params[k] = v
		}
		resp, err := ad.Generate(ctx, req)
		if err != nil {
			markFailed("LLM call failed: " + err.Error())
			return err
		}
		if len(resp.Texts) == 0 {
			markFailed("LLM returned empty result")
			return errors.New("empty LLM response")
		}
		s.publish(topic, "progress", 0.7, "parsing episodes")
		eps, err := parseEpisodes(resp.Texts[0])
		if err != nil {
			markFailed("parse failed: " + err.Error())
			return err
		}
		if len(eps) == 0 {
			markFailed("no episodes parsed")
			return errors.New("no episodes parsed")
		}
		for i := range eps {
			eps[i].ScriptID = p.ScriptID
			if eps[i].EpNo == 0 {
				eps[i].EpNo = i + 1
			}
		}
		// 覆盖式写入
		_ = s.r.Episode.DeleteByScript(ctx, p.ScriptID)
		if err := s.r.Episode.BulkCreate(ctx, eps); err != nil {
			markFailed("save failed: " + err.Error())
			return err
		}
		if err := s.r.Script.UpdateStatus(ctx, p.ScriptID, 3); err != nil {
			s.log.Warn("update script status", zap.Error(err))
		}
		s.publish(topic, "done", 1.0, fmt.Sprintf("split into %d episodes", len(eps)))
		return nil
	}
}

// publish 仅在 hub 可用时投递事件
func (s *ScriptService) publish(topic, evType string, pct float64, msg string) {
	if s.hub == nil {
		return
	}
	s.hub.Publish(topic, ws.Event{Type: evType, Percent: pct, Message: msg})
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
		return m[1]
	}
	return s
}

// ============== Prompt ==============

type PromptService struct {
	r   *repo.Repositories
	tc  *queue.Client
	hub *ws.Hub
	log *zap.Logger
}

type GeneratePromptInput struct {
	ModelID int64          `json:"model_id" binding:"required"`
	Params  map[string]any `json:"params"`
}

type promptPayload struct {
	EpisodeID int64          `json:"episode_id"`
	ModelID   int64          `json:"model_id"`
	Params    map[string]any `json:"params"`
	UserID    int64          `json:"user_id"`
}

func (s *PromptService) Generate(ctx context.Context, episodeID int64, in *GeneratePromptInput, uid int64) (string, error) {
	if _, err := s.r.Episode.Get(ctx, episodeID); err != nil {
		return "", errcode.ErrNotFound
	}
	payload, _ := json.Marshal(promptPayload{
		EpisodeID: episodeID,
		ModelID:   in.ModelID,
		Params:    in.Params,
		UserID:    uid,
	})
	return s.tc.Enqueue(ctx, TaskPromptGenerate, payload, asynq.Queue("default"), asynq.MaxRetry(3))
}

func (s *PromptService) ListByEpisode(ctx context.Context, episodeID int64) ([]model.EpisodePrompt, error) {
	return s.r.Prompt.ListByEpisode(ctx, episodeID)
}

func (s *PromptService) GetCurrent(ctx context.Context, episodeID int64) (*model.EpisodePrompt, error) {
	return s.r.Prompt.GetCurrent(ctx, episodeID)
}

func (s *PromptService) SetCurrent(ctx context.Context, episodeID, id int64) error {
	return s.r.Prompt.SetCurrent(ctx, episodeID, id)
}

func (s *PromptService) HandleGenerateTask(modelSvc *ModelService) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p promptPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		topic := fmt.Sprintf("episode:%d", p.EpisodeID)
		s.publish(topic, "progress", 0.05, "fetching episode")

		ep, err := s.r.Episode.Get(ctx, p.EpisodeID)
		if err != nil {
			s.publish(topic, "error", 0, "episode not found")
			return err
		}
		ad, m, err := modelSvc.GetAdapter(ctx, p.ModelID)
		if err != nil {
			s.publish(topic, "error", 0, "model adapter unavailable: "+err.Error())
			return err
		}
		if ad.Type() != adapter.TypeText {
			s.publish(topic, "error", 0, "prompt generation requires a text model")
			return errors.New("prompt.generate requires text model")
		}
		s.publish(topic, "progress", 0.3, "calling LLM "+m.Code)

		prompt := buildPromptGenPrompt(ep)
		req := &adapter.Request{
			Prompt: prompt,
			Params: map[string]any{
				"system":      "你是一个短剧分镜与提示词专家,输出严格 JSON,不要解释。",
				"temperature": 0.7,
			},
		}
		for k, v := range p.Params {
			req.Params[k] = v
		}
		resp, err := ad.Generate(ctx, req)
		if err != nil {
			s.publish(topic, "error", 0, "LLM call failed: "+err.Error())
			return err
		}
		if len(resp.Texts) == 0 {
			s.publish(topic, "error", 0, "LLM returned empty result")
			return errors.New("empty LLM response")
		}
		content, err := parsePromptJSON(resp.Texts[0])
		if err != nil {
			s.publish(topic, "error", 0, "parse failed: "+err.Error())
			return err
		}
		s.publish(topic, "progress", 0.85, "saving prompt")
		ep0 := &model.EpisodePrompt{
			EpisodeID:        p.EpisodeID,
			Content:          model.JSON(content),
			ModelID:          p.ModelID,
			GenerationParams: toJSON(p.Params),
			Status:           1,
			GeneratedBy:      p.UserID,
		}
		if err := s.r.Prompt.CreateAsCurrent(ctx, ep0); err != nil {
			s.publish(topic, "error", 0, "save failed: "+err.Error())
			return err
		}
		s.publish(topic, "done", 1.0, "prompt generated")
		return nil
	}
}

func (s *PromptService) publish(topic, evType string, pct float64, msg string) {
	if s.hub == nil {
		return
	}
	s.hub.Publish(topic, ws.Event{Type: evType, Percent: pct, Message: msg})
}

func buildPromptGenPrompt(ep *model.Episode) string {
	var sb strings.Builder
	sb.WriteString("基于下面这一集剧本,生成完整的分镜级提示词,JSON 结构如下:\n")
	sb.WriteString(`{"summary":"...", "shots":[{"shot_no":1,"shot_type":"medium|close|wide","camera":"static|push|pan","scene":"...","characters":["A","B"],"action":"...","dialogue":"...","duration_sec":15,"image_prompt":"...","video_prompt":"..."}]}`)
	sb.WriteString("\n要求镜头数 6-12,每个 shot_no 顺序递增,image_prompt 必须含正向描述与风格关键词。\n\n")
	sb.WriteString(fmt.Sprintf("第 %d 集 - %s\n摘要: %s\n剧本:\n%s\n", ep.EpNo, ep.Title, ep.Summary, ep.RawSegment))
	return sb.String()
}

func parsePromptJSON(out string) ([]byte, error) {
	s := stripJSONFence(out)
	l := strings.Index(s, "{")
	r := strings.LastIndex(s, "}")
	if l < 0 || r <= l {
		return nil, errors.New("no JSON object found")
	}
	raw := s[l : r+1]
	// 验证可被解码
	var probe map[string]any
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

// ============== Storyboard / Image / ShortVideo / Full / Pipeline 占位 ==============

type StoryboardService struct {
	r   *repo.Repositories
	tc  *queue.Client
	hub *ws.Hub
	log *zap.Logger
}

type storyboardPayload struct {
	EpisodeID int64          `json:"episode_id"`
	ModelID   int64          `json:"model_id"`
	Params    map[string]any `json:"params"`
	UserID    int64          `json:"user_id"`
}

func (s *StoryboardService) ListByEpisode(ctx context.Context, episodeID int64) ([]model.Storyboard, error) {
	return s.r.Story.ListByEpisode(ctx, episodeID)
}

func (s *StoryboardService) Get(ctx context.Context, id int64) (*model.Storyboard, error) {
	return s.r.Story.Get(ctx, id)
}

func (s *StoryboardService) Update(ctx context.Context, sb *model.Storyboard) error {
	return s.r.Story.Update(ctx, sb)
}

func (s *StoryboardService) Delete(ctx context.Context, id int64) error {
	return s.r.Story.Delete(ctx, id)
}

// BulkSave 覆盖式批量保存某集分镜
func (s *StoryboardService) BulkSave(ctx context.Context, episodeID int64, list []model.Storyboard) error {
	for i := range list {
		list[i].EpisodeID = episodeID
	}
	if err := s.r.Story.DeleteByEpisode(ctx, episodeID); err != nil {
		return err
	}
	return s.r.Story.BulkCreate(ctx, list)
}

func (s *StoryboardService) ApplyStyle(ctx context.Context, storyboardID, styleID, userID int64) error {
	return s.r.Story.ApplyStyle(ctx, storyboardID, styleID, userID)
}

// Generate 异步基于已存在的 prompt 生成分镜
func (s *StoryboardService) Generate(ctx context.Context, episodeID, modelID int64, params map[string]any, uid int64) (string, error) {
	payload, _ := json.Marshal(storyboardPayload{
		EpisodeID: episodeID,
		ModelID:   modelID,
		Params:    params,
		UserID:    uid,
	})
	return s.tc.Enqueue(ctx, TaskStoryboardGenerate, payload, asynq.Queue("default"), asynq.MaxRetry(3))
}

// HandleGenerateTask 把 episode 当前 prompt JSON 的 shots[] 转换为 storyboard 行
func (s *StoryboardService) HandleGenerateTask(modelSvc *ModelService) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p storyboardPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		topic := fmt.Sprintf("episode:%d", p.EpisodeID)
		s.publish(topic, "progress", 0.1, "loading current prompt")

		cur, err := s.r.Prompt.GetCurrent(ctx, p.EpisodeID)
		if err != nil {
			s.publish(topic, "error", 0, "no current prompt")
			return err
		}
		shots, err := parsePromptShots([]byte(cur.Content))
		if err != nil || len(shots) == 0 {
			s.publish(topic, "error", 0, "prompt has no shots[]")
			return errors.New("prompt has no shots")
		}
		s.publish(topic, "progress", 0.5, fmt.Sprintf("parsed %d shots", len(shots)))

		list := make([]model.Storyboard, 0, len(shots))
		for _, sh := range shots {
			chs, _ := json.Marshal(sh.Characters)
			list = append(list, model.Storyboard{
				EpisodeID:    p.EpisodeID,
				PromptID:     cur.ID,
				ShotNo:       sh.ShotNo,
				ShotType:     defaultStr(sh.ShotType, "medium"),
				CameraMotion: defaultStr(sh.Camera, "static"),
				SceneDesc:    sh.Scene,
				Characters:   model.JSON(chs),
				Action:       sh.Action,
				Dialogue:     sh.Dialogue,
				DurationSec:  defaultInt(sh.DurationSec, 15),
				Status:       1,
			})
		}
		if err := s.BulkSave(ctx, p.EpisodeID, list); err != nil {
			s.publish(topic, "error", 0, "save failed: "+err.Error())
			return err
		}
		s.publish(topic, "done", 1.0, fmt.Sprintf("storyboard ready (%d shots)", len(list)))
		return nil
	}
}

type promptShot struct {
	ShotNo      int      `json:"shot_no"`
	ShotType    string   `json:"shot_type"`
	Camera      string   `json:"camera"`
	Scene       string   `json:"scene"`
	Characters  []string `json:"characters"`
	Action      string   `json:"action"`
	Dialogue    string   `json:"dialogue"`
	DurationSec int      `json:"duration_sec"`
	ImagePrompt string   `json:"image_prompt"`
	VideoPrompt string   `json:"video_prompt"`
}

func parsePromptShots(raw []byte) ([]promptShot, error) {
	var doc struct {
		Shots []promptShot `json:"shots"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc.Shots, nil
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
func defaultInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func (s *StoryboardService) publish(topic, evType string, pct float64, msg string) {
	if s.hub == nil {
		return
	}
	s.hub.Publish(topic, ws.Event{Type: evType, Percent: pct, Message: msg})
}

// ============== ImageService ==============

type ImageService struct {
	r     *repo.Repositories
	tc    *queue.Client
	hub   *ws.Hub
	store storage.Storage
	log   *zap.Logger
}

type ImageGenInput struct {
	StoryboardID int64          `json:"storyboard_id"`
	ProjectID    int64          `json:"project_id"`
	StyleID      int64          `json:"style_id"`
	ModelID      int64          `json:"model_id" binding:"required"`
	Prompt       string         `json:"prompt"`
	NegPrompt    string         `json:"neg_prompt"`
	Params       map[string]any `json:"params"`
}

type imagePayload struct {
	StoryboardID int64          `json:"storyboard_id"`
	ProjectID    int64          `json:"project_id"`
	StyleID      int64          `json:"style_id"`
	ModelID      int64          `json:"model_id"`
	Prompt       string         `json:"prompt"`
	NegPrompt    string         `json:"neg_prompt"`
	Params       map[string]any `json:"params"`
	UserID       int64          `json:"user_id"`
	DeptID       int64          `json:"dept_id"`
}

func (s *ImageService) List(ctx context.Context, q *repo.ListImagesQuery) ([]model.Image, int64, error) {
	return s.r.Image.List(ctx, q)
}
func (s *ImageService) Get(ctx context.Context, id int64) (*model.Image, error) {
	return s.r.Image.Get(ctx, id)
}
func (s *ImageService) Delete(ctx context.Context, id int64) error {
	return s.r.Image.Delete(ctx, id)
}

func (s *ImageService) Generate(ctx context.Context, in *ImageGenInput, uid, deptID int64) (string, error) {
	payload, _ := json.Marshal(imagePayload{
		StoryboardID: in.StoryboardID,
		ProjectID:    in.ProjectID,
		StyleID:      in.StyleID,
		ModelID:      in.ModelID,
		Prompt:       in.Prompt,
		NegPrompt:    in.NegPrompt,
		Params:       in.Params,
		UserID:       uid,
		DeptID:       deptID,
	})
	return s.tc.Enqueue(ctx, TaskImageGenerate, payload, asynq.Queue("default"), asynq.MaxRetry(3))
}

func (s *ImageService) HandleGenerateTask(modelSvc *ModelService, invSvc *InvocationService) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p imagePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		topic := fmt.Sprintf("image:%d", p.StoryboardID)
		s.publish(topic, "progress", 0.05, "preparing image generation")
		start := time.Now()

		ad, m, err := modelSvc.GetAdapter(ctx, p.ModelID)
		if err != nil {
			s.publish(topic, "error", 0, "image model unavailable: "+err.Error())
			endErr := time.Now()
			invSvc.Log(ctx, &LogParams{
				ModelID: p.ModelID, UserID: p.UserID, DeptID: p.DeptID, ProjectID: p.ProjectID,
				BizType: "image_gen", BizRef: fmt.Sprintf("storyboard:%d", p.StoryboardID),
				Status: "failed", ErrorCode: "adapter_unavailable",
				StartedAt: start, EndedAt: &endErr,
				DurationMs: int(endErr.Sub(start) / time.Millisecond),
			})
			return err
		}
		if ad.Type() != adapter.TypeImage {
			s.publish(topic, "error", 0, "model is not an image model")
			endErr := time.Now()
			invSvc.Log(ctx, &LogParams{
				ModelID: p.ModelID, UserID: p.UserID, DeptID: p.DeptID, ProjectID: p.ProjectID,
				BizType: "image_gen", BizRef: fmt.Sprintf("storyboard:%d", p.StoryboardID),
				Status: "failed", ErrorCode: "model_type_mismatch",
				StartedAt: start, EndedAt: &endErr,
				DurationMs: int(endErr.Sub(start) / time.Millisecond),
			})
			return errors.New("image.generate requires an image model")
		}

		fullPrompt := p.Prompt
		if p.StyleID > 0 {
			if st, _ := s.r.Style.Get(ctx, p.StyleID); st != nil {
				fullPrompt = buildStyledPrompt(p.Prompt, st)
			}
		}
		s.publish(topic, "progress", 0.3, "calling image model "+m.Code)
		req := &adapter.Request{
			Prompt:    fullPrompt,
			NegPrompt: p.NegPrompt,
			Params:    p.Params,
		}
		resp, err := ad.Generate(ctx, req)
		if err != nil {
			s.publish(topic, "error", 0, "image model failed: "+err.Error())
			endErr := time.Now()
			invSvc.Log(ctx, &LogParams{
				ModelID: p.ModelID, UserID: p.UserID, DeptID: p.DeptID, ProjectID: p.ProjectID,
				BizType: "image_gen", BizRef: fmt.Sprintf("storyboard:%d", p.StoryboardID),
				Status: "failed", ErrorCode: "model_call_failed",
				StartedAt: start, EndedAt: &endErr,
				DurationMs: int(endErr.Sub(start) / time.Millisecond),
			})
			return err
		}
		if len(resp.ImageURLs) == 0 {
			s.publish(topic, "error", 0, "image model returned no URL")
			endErr := time.Now()
			invSvc.Log(ctx, &LogParams{
				ModelID: p.ModelID, UserID: p.UserID, DeptID: p.DeptID, ProjectID: p.ProjectID,
				BizType: "image_gen", BizRef: fmt.Sprintf("storyboard:%d", p.StoryboardID),
				Status: "failed", ErrorCode: "empty_image_url",
				StartedAt: start, EndedAt: &endErr,
				DurationMs: int(endErr.Sub(start) / time.Millisecond),
			})
			return errors.New("empty image response")
		}
		s.publish(topic, "progress", 0.7, "persisting image asset")
		// 把远程 URL 拉下来落到对象存储,得到稳定内部 URL
		stableURL := persistRemoteAsset(ctx, s.store, "images", resp.ImageURLs[0], s.log)
		s.publish(topic, "progress", 0.85, "saving image record")
		paramJSON, _ := json.Marshal(p.Params)
		img := &model.Image{
			ProjectID:    p.ProjectID,
			StoryboardID: p.StoryboardID,
			SrcType:      "generated",
			URL:          stableURL,
			Width:        toIntFromAny(resp.Raw["width"]),
			Height:       toIntFromAny(resp.Raw["height"]),
			Prompt:       fullPrompt,
			NegPrompt:    p.NegPrompt,
			ModelID:      p.ModelID,
			Params:       model.JSON(paramJSON),
			Status:       2, // succeeded
			CreatedBy:    p.UserID,
		}
		img.CreatedBy = p.UserID
		if err := s.r.Image.Create(ctx, img); err != nil {
			s.publish(topic, "error", 0, "persist image failed: "+err.Error())
			return err
		}
		end := time.Now()
		invSvc.Log(ctx, &LogParams{
			ModelID:    p.ModelID,
			UserID:     p.UserID,
			DeptID:     p.DeptID,
			ProjectID:  p.ProjectID,
			BizType:    "image_gen",
			BizRef:     fmt.Sprintf("image:%d", img.ID),
			Units:      max1(len(resp.ImageURLs)),
			DurationMs: int(end.Sub(start) / time.Millisecond),
			Status:     "succeeded",
			StartedAt:  start,
			EndedAt:    &end,
		})
		s.publish(topic, "done", 1.0, fmt.Sprintf("image #%d ready", img.ID))
		return nil
	}
}

func (s *ImageService) publish(topic, evType string, pct float64, msg string) {
	if s.hub == nil {
		return
	}
	s.hub.Publish(topic, ws.Event{Type: evType, Percent: pct, Message: msg})
}

func buildStyledPrompt(base string, st *model.Style) string {
	var sb strings.Builder
	sb.WriteString(base)
	if st.ArtStyle != "" {
		sb.WriteString(", art_style: ")
		sb.WriteString(st.ArtStyle)
	}
	if st.ColorTone != "" {
		sb.WriteString(", color_tone: ")
		sb.WriteString(st.ColorTone)
	}
	if st.Lighting != "" {
		sb.WriteString(", lighting: ")
		sb.WriteString(st.Lighting)
	}
	if st.Description != "" {
		sb.WriteString(", style_note: ")
		sb.WriteString(st.Description)
	}
	return sb.String()
}

func toIntFromAny(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// ============== ShortVideoService ==============

type ShortVideoService struct {
	r     *repo.Repositories
	tc    *queue.Client
	hub   *ws.Hub
	store storage.Storage
	log   *zap.Logger
}

type ShortVideoGenInput struct {
	StoryboardID   int64          `json:"storyboard_id"`
	ProjectID      int64          `json:"project_id"`
	SourceImageIDs []int64        `json:"source_image_ids"`
	Prompt         string         `json:"prompt"`
	ModelID        int64          `json:"model_id" binding:"required"`
	Params         map[string]any `json:"params"`
}

type shortVideoPayload struct {
	StoryboardID   int64          `json:"storyboard_id"`
	ProjectID      int64          `json:"project_id"`
	SourceImageIDs []int64        `json:"source_image_ids"`
	Prompt         string         `json:"prompt"`
	ModelID        int64          `json:"model_id"`
	Params         map[string]any `json:"params"`
	UserID         int64          `json:"user_id"`
	DeptID         int64          `json:"dept_id"`
}

func (s *ShortVideoService) List(ctx context.Context, q *repo.ListShortVideosQuery) ([]model.ShortVideo, int64, error) {
	return s.r.Short.List(ctx, q)
}
func (s *ShortVideoService) Get(ctx context.Context, id int64) (*model.ShortVideo, error) {
	return s.r.Short.Get(ctx, id)
}
func (s *ShortVideoService) Delete(ctx context.Context, id int64) error {
	return s.r.Short.Delete(ctx, id)
}

func (s *ShortVideoService) Generate(ctx context.Context, in *ShortVideoGenInput, uid, deptID int64) (string, error) {
	payload, _ := json.Marshal(shortVideoPayload{
		StoryboardID:   in.StoryboardID,
		ProjectID:      in.ProjectID,
		SourceImageIDs: in.SourceImageIDs,
		Prompt:         in.Prompt,
		ModelID:        in.ModelID,
		Params:         in.Params,
		UserID:         uid,
		DeptID:         deptID,
	})
	return s.tc.Enqueue(ctx, TaskVideoGenerate, payload, asynq.Queue("default"), asynq.MaxRetry(3))
}

func (s *ShortVideoService) HandleGenerateTask(modelSvc *ModelService, invSvc *InvocationService) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p shortVideoPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		topic := fmt.Sprintf("short:%d", p.StoryboardID)
		s.publish(topic, "progress", 0.05, "preparing video generation")
		start := time.Now()

		// 先建占位记录,worker 后续更新它的 URL/状态
		srcJSON, _ := json.Marshal(p.SourceImageIDs)
		paramJSON, _ := json.Marshal(p.Params)
		sv := &model.ShortVideo{
			ProjectID:      p.ProjectID,
			StoryboardID:   p.StoryboardID,
			SrcType:        "generated",
			Prompt:         p.Prompt,
			SourceImageIDs: model.JSON(srcJSON),
			ModelID:        p.ModelID,
			Params:         model.JSON(paramJSON),
			Status:         "running",
		}
		sv.CreatedBy = p.UserID
		if err := s.r.Short.Create(ctx, sv); err != nil {
			s.publish(topic, "error", 0, "persist short video init failed: "+err.Error())
			return err
		}

		ad, m, err := modelSvc.GetAdapter(ctx, p.ModelID)
		if err != nil {
			_ = s.r.Short.UpdateStatus(ctx, sv.ID, "failed", err.Error())
			s.publish(topic, "error", 0, "video model unavailable: "+err.Error())
			endErr := time.Now()
			invSvc.Log(ctx, &LogParams{
				ModelID: p.ModelID, UserID: p.UserID, DeptID: p.DeptID, ProjectID: p.ProjectID,
				BizType: "video_gen", BizRef: fmt.Sprintf("short:%d", sv.ID),
				Status: "failed", ErrorCode: "adapter_unavailable",
				StartedAt: start, EndedAt: &endErr,
				DurationMs: int(endErr.Sub(start) / time.Millisecond),
			})
			return err
		}
		if ad.Type() != adapter.TypeVideo {
			_ = s.r.Short.UpdateStatus(ctx, sv.ID, "failed", "not a video model")
			s.publish(topic, "error", 0, "model is not a video model")
			endErr := time.Now()
			invSvc.Log(ctx, &LogParams{
				ModelID: p.ModelID, UserID: p.UserID, DeptID: p.DeptID, ProjectID: p.ProjectID,
				BizType: "video_gen", BizRef: fmt.Sprintf("short:%d", sv.ID),
				Status: "failed", ErrorCode: "model_type_mismatch",
				StartedAt: start, EndedAt: &endErr,
				DurationMs: int(endErr.Sub(start) / time.Millisecond),
			})
			return errors.New("video.generate requires a video model")
		}
		s.publish(topic, "progress", 0.3, "calling video model "+m.Code)

		// 把入参中的图片 URL 注入到 inputs
		inputs := make([]string, 0, len(p.SourceImageIDs))
		for _, id := range p.SourceImageIDs {
			if img, _ := s.r.Image.Get(ctx, id); img != nil && img.URL != "" {
				inputs = append(inputs, img.URL)
			}
		}
		req := &adapter.Request{
			Prompt: p.Prompt,
			Inputs: inputs,
			Params: p.Params,
		}
		resp, err := ad.Generate(ctx, req)
		if err != nil {
			_ = s.r.Short.UpdateStatus(ctx, sv.ID, "failed", err.Error())
			s.publish(topic, "error", 0, "video model call failed: "+err.Error())
			endErr := time.Now()
			invSvc.Log(ctx, &LogParams{
				ModelID: p.ModelID, UserID: p.UserID, DeptID: p.DeptID, ProjectID: p.ProjectID,
				BizType: "video_gen", BizRef: fmt.Sprintf("short:%d", sv.ID),
				Status: "failed", ErrorCode: "model_call_failed",
				StartedAt: start, EndedAt: &endErr,
				DurationMs: int(endErr.Sub(start) / time.Millisecond),
			})
			return err
		}
		if len(resp.VideoURLs) == 0 {
			_ = s.r.Short.UpdateStatus(ctx, sv.ID, "failed", "no video url")
			s.publish(topic, "error", 0, "video model returned no URL")
			endErr := time.Now()
			invSvc.Log(ctx, &LogParams{
				ModelID: p.ModelID, UserID: p.UserID, DeptID: p.DeptID, ProjectID: p.ProjectID,
				BizType: "video_gen", BizRef: fmt.Sprintf("short:%d", sv.ID),
				Status: "failed", ErrorCode: "empty_video_url",
				StartedAt: start, EndedAt: &endErr,
				DurationMs: int(endErr.Sub(start) / time.Millisecond),
			})
			return errors.New("empty video response")
		}
		s.publish(topic, "progress", 0.75, "persisting video asset")
		stableURL := persistRemoteAsset(ctx, s.store, "videos", resp.VideoURLs[0], s.log)
		sv.VideoURL = stableURL
		sv.DurationMs = resp.DurationMs
		sv.Width = toIntFromAny(resp.Raw["width"])
		sv.Height = toIntFromAny(resp.Raw["height"])
		sv.Status = "succeeded"
		if err := s.r.Short.Update(ctx, sv); err != nil {
			s.publish(topic, "error", 0, "persist video failed: "+err.Error())
			return err
		}

		end := time.Now()
		invSvc.Log(ctx, &LogParams{
			ModelID:    p.ModelID,
			UserID:     p.UserID,
			DeptID:     p.DeptID,
			ProjectID:  p.ProjectID,
			BizType:    "video_gen",
			BizRef:     fmt.Sprintf("short:%d", sv.ID),
			Units:      sv.DurationMs / 1000,
			DurationMs: int(end.Sub(start) / time.Millisecond),
			Status:     "succeeded",
			StartedAt:  start,
			EndedAt:    &end,
		})
		s.publish(topic, "done", 1.0, fmt.Sprintf("short video #%d ready", sv.ID))
		return nil
	}
}

func (s *ShortVideoService) publish(topic, evType string, pct float64, msg string) {
	if s.hub == nil {
		return
	}
	s.hub.Publish(topic, ws.Event{Type: evType, Percent: pct, Message: msg})
}

// ============== FullVideoService / PipelineService 占位 ==============

type FullVideoService struct {
	r        *repo.Repositories
	tc       *queue.Client
	hub      *ws.Hub
	store    storage.Storage
	modelSvc *ModelService
	invSvc   *InvocationService
	log      *zap.Logger
}

type PipelineService struct {
	r        *repo.Repositories
	tc       *queue.Client
	hub      *ws.Hub
	registry pipelineRegistry // 由 worker 端注入,server 端可为 nil
	log      *zap.Logger
}

// pipelineRegistry 由 pipeline 包提供,这里用接口避免循环 import
type pipelineRegistry interface {
	Execute(ctx context.Context, dagJSON []byte, input map[string]any, runID int64) (map[string]any, error)
}

// SetDeps 由 NewServices 后注入
func (s *PipelineService) SetDeps(hub *ws.Hub, registry pipelineRegistry) {
	s.hub = hub
	s.registry = registry
}

func (s *PipelineService) publish(topic, evType string, pct float64, msg string) {
	if s.hub == nil {
		return
	}
	s.hub.Publish(topic, ws.Event{Type: evType, Percent: pct, Message: msg})
}

// Run 投递 pipeline.run 任务。
//
// 修复 P0 #4:
//   - 先 service 层创建 PipelineRun(status=queued) → 返回 int64 run.ID
//   - 用 asynq.TaskID("pipeline-run-{run.ID}") 保证幂等(同 run.ID 不会重复入队)
//   - 前端拿到 run.ID 后 WS 订阅 pipeline:<run.ID> 才能命中
func (s *PipelineService) Run(ctx context.Context, pipelineID int64, input map[string]any, overrides map[string]any) (int64, error) {
	pl, err := s.r.Pipeline.Get(ctx, pipelineID)
	if err != nil {
		return 0, fmt.Errorf("pipeline.run: load pipeline %d: %w", pipelineID, err)
	}
	if len(pl.DAG) == 0 {
		return 0, fmt.Errorf("pipeline.run: pipeline %d has empty dag", pipelineID)
	}

	inputBytes, _ := json.Marshal(input)
	now := time.Now()
	run := &model.PipelineRun{
		PipelineID:  pl.ID,
		ProjectID:   pl.ProjectID,
		TriggerType: "manual",
		Input:       model.JSON(inputBytes),
		Status:      "queued",
		StartedAt:   &now,
	}
	if err := s.r.Pipeline.CreateRun(ctx, run); err != nil {
		return 0, fmt.Errorf("pipeline.run: create run: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"run_id":      run.ID,
		"pipeline_id": pipelineID,
		"input":       input,
		"overrides":   overrides,
	})
	if _, err := s.tc.Enqueue(ctx, TaskPipelineRun, payload,
		asynq.Queue("critical"),
		asynq.TaskID(fmt.Sprintf("pipeline-run-%d", run.ID)),
		asynq.MaxRetry(3),
	); err != nil {
		_ = s.r.Pipeline.UpdateRunStatus(ctx, run.ID, "failed", "enqueue: "+err.Error())
		return 0, err
	}
	return run.ID, nil
}

// ============== Asynq Task 名称常量 ==============

const (
	TaskScriptSplit        = "script.split"
	TaskPromptGenerate     = "prompt.generate"
	TaskStoryboardGenerate = "storyboard.generate"
	TaskImageGenerate      = "image.generate"
	TaskVideoGenerate      = "video.generate"
	TaskVideoCompose       = "video.compose"
	TaskPipelineRun        = "pipeline.run"
)

// 防止 time 包未使用警告(在某些子集编译时)
var _ = time.Now
