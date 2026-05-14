package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/adapter"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/ffmpeg"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/storage"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/subtitle"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/ws"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// FullVideoService 是 Sprint4 完整视频域的核心服务。
//
// 负责:
//   - CRUD 完整视频记录
//   - 把 timeline (clips[] + tts + subtitles) 提交到 asynq,由 worker 调用 ffmpeg 合成
//   - 进度通过 hub 推送到 topic "full:<id>"
type FullVideoServiceFull struct{}

// TimelineClip 一个时间线片段
type TimelineClip struct {
	ShortVideoID int64  `json:"short_video_id"`
	URL          string `json:"url"`         // 直接给定 URL,可不依赖 short_video_id
	DurationMs   int    `json:"duration_ms"` // 估算时长(用于进度计算与字幕排版)
	TTSText      string `json:"tts_text"`    // 可选:为该片段生成 TTS 配音并烧字幕
	Speaker      string `json:"speaker"`     // 可选:字幕角色前缀
}

// Timeline 完整视频时间线
type Timeline struct {
	Clips           []TimelineClip `json:"clips"`
	BackgroundAudio string         `json:"background_audio_url"` // 可选:外部 BGM URL
	TTSModelID      int64          `json:"tts_model_id"`         // 可选:为 clip.tts_text 生成配音
	BurnSubtitles   bool           `json:"burn_subtitles"`       // 是否烧字幕
}

// CreateFullVideoInput 创建一条完整视频草稿
type CreateFullVideoInput struct {
	ProjectID int64    `json:"project_id" binding:"required,gte=1"`
	Name      string   `json:"name" binding:"required,min=1,max=200"`
	Timeline  Timeline `json:"timeline" binding:"required"`
}

// UpdateFullVideoInput 更新完整视频草稿
type UpdateFullVideoInput struct {
	Name     *string   `json:"name" binding:"omitempty,min=1,max=200"`
	Timeline *Timeline `json:"timeline"`
}

type composePayload struct {
	FullVideoID int64 `json:"full_video_id"`
	UserID      int64 `json:"user_id"`
}

// --- 扩展原有 FullVideoService ---

// SetDeps 在 NewServices 之后由外部注入 hub/store/modelSvc/invSvc 等依赖(避免循环初始化)
func (s *fullVideoService) SetDeps(hub *ws.Hub, store storage.Storage, modelSvc ModelService, invSvc InvocationService) {
	s.hub = hub
	s.store = store
	s.modelSvc = modelSvc
	s.invSvc = invSvc
}

func (s *fullVideoService) publish(topic, evType string, pct float64, msg string) {
	if s.hub == nil {
		return
	}
	s.hub.Publish(topic, ws.Event{Type: evType, Percent: pct, Message: msg})
}

// List 完整视频列表
func (s *fullVideoService) List(ctx context.Context, q *repo.ListFullVideosQuery) ([]model.FullVideo, int64, error) {
	return s.r.Full.List(ctx, q)
}

// Get 完整视频详情
func (s *fullVideoService) Get(ctx context.Context, id int64) (*model.FullVideo, error) {
	f, err := s.r.Full.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return f, nil
}

// Create 新建完整视频
func (s *fullVideoService) Create(ctx context.Context, in *CreateFullVideoInput, uid int64) (*model.FullVideo, error) {
	tlBytes, err := json.Marshal(in.Timeline)
	if err != nil {
		return nil, errcode.ErrParam.Wrap(err)
	}
	f := &model.FullVideo{
		ProjectID: in.ProjectID,
		Name:      in.Name,
		Timeline:  model.JSON(tlBytes),
		Status:    "draft",
		Version:   1,
	}
	f.CreatedBy = uid
	f.UpdatedBy = uid
	if err := s.r.Full.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// Update 更新完整视频元数据/时间线
func (s *fullVideoService) Update(ctx context.Context, id int64, in *UpdateFullVideoInput, uid int64) (*model.FullVideo, error) {
	f, err := s.r.Full.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	if in.Name != nil {
		f.Name = *in.Name
	}
	if in.Timeline != nil {
		tlBytes, err := json.Marshal(in.Timeline)
		if err != nil {
			return nil, errcode.ErrParam.Wrap(err)
		}
		f.Timeline = model.JSON(tlBytes)
	}
	f.UpdatedBy = uid
	if err := s.r.Full.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// Delete 删除完整视频记录
func (s *fullVideoService) Delete(ctx context.Context, id int64) error {
	return s.r.Full.Delete(ctx, id)
}

// HandleComposeTask 是 worker 端 video.compose 任务处理器。
//
// 流程:
//  1. 加载 timeline
//  2. 解析每个 clip 的视频 URL(可选 short_video_id -> URL)
//  3. 可选:为每个 clip.tts_text 生成 TTS 音频(上传到对象存储)
//  4. ffmpeg concat 合并视频
//  5. 可选:overlay 全局音轨(背景音 / 全 TTS 合成的总音轨)
//  6. 可选:从 clip.tts_text + duration 生成 SRT 并烧字幕
//  7. 探测最终时长 + 抽帧封面
//  8. 上传最终视频,更新 FullVideo 状态/URL
func (s *fullVideoService) HandleComposeTask() asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p composePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode compose payload: %w", err)
		}
		topic := fmt.Sprintf("full:%d", p.FullVideoID)
		start := time.Now()
		s.publish(topic, "progress", 0.02, "加载时间线")

		// 为整个视频合成流程设置超时,防止下游卡住导致 goroutine 泄漏
		composeCtx, cancel := context.WithTimeout(ctx, getTimeout("TIMEOUT_VIDEO_COMPOSE", 300))
		defer cancel()
		// 后续外部调用(ffmpeg/TTS/上传)使用 composeCtx,数据库写操作仍用原始 ctx

		full, err := s.r.Full.Get(ctx, p.FullVideoID)
		if err != nil {
			s.publish(topic, "error", 0, "full video not found")
			return err
		}
		_ = s.r.Full.UpdateStatus(ctx, full.ID, "running", 2, "")

		var tl Timeline
		if len(full.Timeline) > 0 {
			if err := json.Unmarshal(full.Timeline, &tl); err != nil {
				s.log.Error("parse timeline failed", zap.Int64("full_id", full.ID), zap.Error(err))
				s.publish(topic, "error", 0, "解析时间线失败")
				_ = s.r.Full.UpdateStatus(ctx, full.ID, "failed", 0, "parse timeline failed")
				return err
			}
		}
		if len(tl.Clips) == 0 {
			s.publish(topic, "error", 0, "timeline 没有任何片段")
			_ = s.r.Full.UpdateStatus(ctx, full.ID, "failed", 0, "no clips")
			return errors.New("no clips")
		}

		if !ffmpeg.Available() {
			msg := "本机未检测到 ffmpeg,无法合成"
			s.publish(topic, "error", 0, msg)
			_ = s.r.Full.UpdateStatus(ctx, full.ID, "failed", 0, msg)
			return errors.New(msg)
		}

		// 1) 解析 clip 视频 URL,补全缺失字段
		// 修复 P0 C3 — 先批量收集 ShortVideoID,一次性 IN 查询,避免循环内 N+1
		s.publish(topic, "progress", 0.08, "解析片段 URL")
		needIDs := make([]int64, 0, len(tl.Clips))
		for _, c := range tl.Clips {
			if c.URL == "" && c.ShortVideoID > 0 {
				needIDs = append(needIDs, c.ShortVideoID)
			}
		}
		svMap := make(map[int64]*model.ShortVideo, len(needIDs))
		if len(needIDs) > 0 {
			var svs []model.ShortVideo
			if err := s.r.DB.WithContext(ctx).Model(&model.ShortVideo{}).
				Where("id IN ?", needIDs).Find(&svs).Error; err == nil {
				for i := range svs {
					svMap[svs[i].ID] = &svs[i]
				}
			}
		}

		clips := make([]TimelineClip, 0, len(tl.Clips))
		inputURLs := make([]string, 0, len(tl.Clips))
		totalDur := 0
		for i := range tl.Clips {
			c := tl.Clips[i]
			if c.URL == "" && c.ShortVideoID > 0 {
				sv, ok := svMap[c.ShortVideoID]
				if !ok || sv == nil || sv.VideoURL == "" {
					msg := fmt.Sprintf("clip %d: 找不到短视频 %d", i, c.ShortVideoID)
					s.publish(topic, "error", 0, msg)
					_ = s.r.Full.UpdateStatus(ctx, full.ID, "failed", 0, msg)
					return errors.New(msg)
				}
				c.URL = sv.VideoURL
				if c.DurationMs == 0 {
					c.DurationMs = sv.DurationMs
				}
			}
			if c.URL == "" {
				msg := fmt.Sprintf("clip %d: 缺少视频 URL", i)
				s.publish(topic, "error", 0, msg)
				_ = s.r.Full.UpdateStatus(ctx, full.ID, "failed", 0, msg)
				return errors.New(msg)
			}
			if c.DurationMs == 0 {
				c.DurationMs = 5000
			}
			clips = append(clips, c)
			inputURLs = append(inputURLs, c.URL)
			totalDur += c.DurationMs
		}

		// 2) TTS:为每个 clip.tts_text 生成 mp3 字节,顺序追加得到全 TTS 音轨
		var ttsMP3 []byte
		var subtitleSRT string
		if tl.BurnSubtitles || tl.TTSModelID > 0 {
			s.publish(topic, "progress", 0.18, "生成配音/字幕")
			ttsBuilder := subtitle.NewBuilder()
			audioChunks := make([][]byte, 0, len(clips))
			for i, c := range clips {
				dur := time.Duration(c.DurationMs) * time.Millisecond
				if c.TTSText == "" {
					ttsBuilder.Append("", c.Speaker, dur)
					if tl.TTSModelID > 0 {
						audioChunks = append(audioChunks, silentMP3Placeholder(c.DurationMs))
					}
					continue
				}
				ttsBuilder.Append(c.TTSText, c.Speaker, dur)
				if tl.TTSModelID > 0 && s.modelSvc != nil {
					ad, _, err := s.modelSvc.GetAdapter(composeCtx, tl.TTSModelID)
					if err == nil && ad.Type() == adapter.TypeAudio {
						resp, err := ad.Generate(composeCtx, &adapter.Request{Prompt: c.TTSText})
						if err == nil && resp.Raw != nil {
							if b, ok := resp.Raw["audio_bytes"].([]byte); ok && len(b) > 0 {
								audioChunks = append(audioChunks, b)
								s.publish(topic, "progress", 0.18+0.18*float64(i+1)/float64(len(clips)),
									fmt.Sprintf("配音生成 %d/%d", i+1, len(clips)))
								continue
							}
						}
						s.log.Warn("tts failed", zap.Int64("model_id", tl.TTSModelID), zap.Error(err))
					}
					audioChunks = append(audioChunks, silentMP3Placeholder(c.DurationMs))
				}
			}
			subtitleSRT = ttsBuilder.SRT()
			if len(audioChunks) > 0 {
				ttsMP3 = bytes.Join(audioChunks, nil)
			}
		}

		// 3) 拼接视频
		workDir, err := os.MkdirTemp("", "full-compose-*")
		if err != nil {
			s.log.Error("create temp dir failed", zap.Int64("full_id", full.ID), zap.Error(err))
			s.publish(topic, "error", 0, "无法创建临时目录")
			_ = s.r.Full.UpdateStatus(ctx, full.ID, "failed", 0, "create temp dir failed")
			return err
		}
		defer os.RemoveAll(workDir)

		concatOut := filepath.Join(workDir, "concat.mp4")
		s.publish(topic, "progress", 0.42, "ffmpeg 拼接视频")
		concatProgress := func(pct float64, msg string) {
			// 拼接占总进度 [0.42, 0.65]
			real := 0.42 + (0.65-0.42)*pct
			s.publish(topic, "progress", real, msg)
			_ = s.r.Full.UpdateStatus(ctx, full.ID, "running", int(real*100), "")
		}
		if err := ffmpeg.ConcatVideos(composeCtx, inputURLs, concatOut, totalDur, concatProgress); err != nil {
			s.log.Error("ffmpeg concat failed", zap.Int64("full_id", full.ID), zap.Error(err))
			s.publish(topic, "error", 0, "视频拼接失败")
			_ = s.r.Full.UpdateStatus(ctx, full.ID, "failed", 0, "ffmpeg concat failed")
			return err
		}

		current := concatOut

		// 4) 叠加音频(BGM 或 TTS 合成)
		if tl.BackgroundAudio != "" {
			s.publish(topic, "progress", 0.70, "叠加背景音乐")
			out := filepath.Join(workDir, "with-bgm.mp4")
			if err := ffmpeg.MixAudio(composeCtx, current, tl.BackgroundAudio, out, totalDur, nil); err != nil {
				s.log.Warn("mix BGM failed, continue without it", zap.Error(err))
			} else {
				current = out
			}
		}
		if len(ttsMP3) > 0 {
			s.publish(topic, "progress", 0.75, "叠加配音音轨")
			ttsPath := filepath.Join(workDir, "tts.mp3")
			if err := os.WriteFile(ttsPath, ttsMP3, 0o644); err != nil {
				s.log.Warn("write tts mp3 failed", zap.Error(err))
			} else {
				out := filepath.Join(workDir, "with-tts.mp4")
				if err := ffmpeg.MixAudio(composeCtx, current, ttsPath, out, totalDur, nil); err != nil {
					s.log.Warn("mix TTS failed", zap.Error(err))
				} else {
					current = out
				}
			}
		}

		// 5) 烧字幕
		if tl.BurnSubtitles && subtitleSRT != "" {
			s.publish(topic, "progress", 0.82, "烧录字幕")
			srtPath := filepath.Join(workDir, "subs.srt")
			if err := os.WriteFile(srtPath, []byte(subtitleSRT), 0o644); err != nil {
				s.log.Warn("write srt failed", zap.Error(err))
			} else {
				out := filepath.Join(workDir, "final.mp4")
				if err := ffmpeg.BurnSubtitles(composeCtx, current, srtPath, out, totalDur, nil); err != nil {
					s.log.Warn("burn subtitles failed", zap.Error(err))
				} else {
					current = out
				}
			}
		}

		// 6) 探测 + 抽封面
		s.publish(topic, "progress", 0.88, "提取封面")
		thumbPath := filepath.Join(workDir, "cover.jpg")
		_ = ffmpeg.ExtractThumb(composeCtx, current, thumbPath)
		probe, _ := ffmpeg.Probe(composeCtx, current)

		// 7) 上传最终视频 + 封面到对象存储
		s.publish(topic, "progress", 0.92, "上传成片")
		ts := time.Now().UnixNano()
		videoKey := fmt.Sprintf("full_videos/%d/%d.mp4", full.ID, ts)
		coverKey := fmt.Sprintf("full_videos/%d/%d-cover.jpg", full.ID, ts)
		videoURL, err := putFile(composeCtx, s.store, videoKey, current, "video/mp4")
		if err != nil {
			s.log.Error("upload video failed", zap.Int64("full_id", full.ID), zap.Error(err))
			s.publish(topic, "error", 0, "上传视频失败")
			_ = s.r.Full.UpdateStatus(ctx, full.ID, "failed", 0, "upload failed")
			return err
		}
		coverURL := ""
		if _, err := os.Stat(thumbPath); err == nil {
			coverURL, _ = putFile(composeCtx, s.store, coverKey, thumbPath, "image/jpeg")
		}

		// 8) 上传字幕(可选)与 BGM 不上传(用户提供)
		subURL := ""
		if subtitleSRT != "" {
			subKey := fmt.Sprintf("full_videos/%d/%d.srt", full.ID, ts)
			subURL, _ = putBytes(composeCtx, s.store, subKey, []byte(subtitleSRT), "application/x-subrip")
		}

		// 9) 更新记录
		full.OutputURL = videoURL
		if coverURL != "" {
			full.CoverURL = coverURL
			full.ThumbURL = coverURL
		}
		if subURL != "" {
			// 把字幕 URL 写到 timeline 上,前端可下载
			if tlBytes, err := json.Marshal(map[string]any{
				"clips":                tl.Clips,
				"background_audio_url": tl.BackgroundAudio,
				"tts_model_id":         tl.TTSModelID,
				"burn_subtitles":       tl.BurnSubtitles,
				"subtitle_url":         subURL,
			}); err == nil {
				full.Timeline = model.JSON(tlBytes)
			}
		}
		if probe != nil && probe.DurationMs > 0 {
			full.DurationMs = probe.DurationMs
		} else {
			full.DurationMs = totalDur
		}
		full.Status = "succeeded"
		full.RenderProgress = 100
		full.ErrorMsg = ""
		if err := s.r.Full.Update(ctx, full); err != nil {
			s.log.Error("save full video record failed", zap.Int64("full_id", full.ID), zap.Error(err))
			s.publish(topic, "error", 0, "保存视频记录失败")
			return err
		}

		// 10) 记录调用日志(只记录 compose 这一步,不计成本)
		if s.invSvc != nil {
			end := time.Now()
			s.invSvc.Log(ctx, &LogParams{
				BizType: "video_compose", BizRef: fmt.Sprintf("full:%d", full.ID),
				Units:      max1(len(clips)),
				DurationMs: int(end.Sub(start) / time.Millisecond),
				Status:     "succeeded",
				StartedAt:  start, EndedAt: &end,
			})
		}
		s.publish(topic, "done", 1.0, fmt.Sprintf("合成完成: %s", videoURL))
		return nil
	}
}

// putFile 把本地文件流上传到对象存储,返回 URL
func putFile(ctx context.Context, store storage.Storage, key, localPath, mime string) (string, error) {
	if store == nil {
		return "", errors.New("storage not configured")
	}
	f, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, _ := f.Stat()
	size := int64(0)
	if st != nil {
		size = st.Size()
	}
	return store.Put(ctx, key, f, size, mime)
}

func putBytes(ctx context.Context, store storage.Storage, key string, b []byte, mime string) (string, error) {
	if store == nil {
		return "", errors.New("storage not configured")
	}
	return store.Put(ctx, key, bytes.NewReader(b), int64(len(b)), mime)
}

// silentMP3Placeholder 在 TTS 不可用时生成一段静音 MP3,长度按 ms 估算。
// 使用 MPEG-1 Layer 3 44100Hz stereo 128kbps 帧头,零填充 payload,
// ffmpeg 解码零 payload 为静音。每帧约 26.12ms。
func silentMP3Placeholder(durationMs int) []byte {
	if durationMs <= 0 {
		return nil
	}
	const (
		frameSize       = 418 // 含 4-byte header + 414-byte payload
		frameDurationMs = 26.122
	)
	frames := int(float64(durationMs)/frameDurationMs + 0.5)
	if frames < 1 {
		frames = 1
	}
	out := make([]byte, frames*frameSize)
	for i := 0; i < frames; i++ {
		off := i * frameSize
		out[off+0] = 0xFF
		out[off+1] = 0xFB
		out[off+2] = 0x92 // 128kbps, 44100Hz, padding=1
		out[off+3] = 0x00 // stereo
		// payload already zero
	}
	return out
}

// Render 由 HTTP 入口调用,把任务投递给 worker
func (s *fullVideoService) Render(ctx context.Context, fullID, uid int64) (string, error) {
	if _, err := s.r.Full.Get(ctx, fullID); err != nil {
		return "", errcode.ErrNotFound
	}
	_ = s.r.Full.UpdateStatus(ctx, fullID, "queued", 0, "")
	payload, err := json.Marshal(composePayload{FullVideoID: fullID, UserID: uid})
	if err != nil {
		return "", err
	}
	return s.tc.Enqueue(ctx, TaskVideoCompose, payload, asynq.Queue("default"), asynq.MaxRetry(2))
}

// 防 unused 警告
var _ = strings.Split
