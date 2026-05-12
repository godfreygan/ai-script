package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PublishService 管理 FullVideo 的发布记录(publishes 表)。
//
// 发布的前置条件:
//   - 对应 FullVideo 存在
//   - FullVideo.Status == "succeeded"
//   - 若同一 FullVideo 已有 status=on 的发布记录,拒绝重复发布(避免重置计数)
//
// (可选)若存在活跃的 ReviewRecord,其 Status 必须为 "approved"。
type PublishService struct {
	r   *repo.Repositories
	log *zap.Logger
}

// PublishInput 发布入参
type PublishInput struct {
	FullVideoID     int64           `json:"full_video_id" binding:"required"`
	WatermarkConfig json.RawMessage `json:"watermark_config"`
}

// validateJSONObject 确保入参是合法 JSON object(不允许数组/字面量/null/空串)。
// 空 RawMessage 视为未传,允许。
func validateJSONObject(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return errcode.ErrParam.WithMsg("watermark_config 必须是合法 JSON object")
	}
	if obj == nil {
		return errcode.ErrParam.WithMsg("watermark_config 不能为 null")
	}
	return nil
}

// Publish 发布一条完整视频。
// 前置条件:
//  1. FullVideo 存在且 Status == "succeeded"
//  2. 不存在 status=on 的活跃发布(避免重复发布、重置 play/download 计数)
//  3. 若存在 active 审核记录,需 approved
//  4. WatermarkConfig 若传则必须是合法 JSON object
func (s *PublishService) Publish(ctx context.Context, in *PublishInput, uid int64) (*model.Publish, error) {
	if in == nil || in.FullVideoID <= 0 {
		return nil, errcode.ErrParam.WithMsg("full_video_id 必填")
	}
	if err := validateJSONObject(in.WatermarkConfig); err != nil {
		return nil, err
	}

	full, err := s.r.Full.Get(ctx, in.FullVideoID)
	if err != nil || full == nil {
		return nil, errcode.ErrNotFound
	}
	if full.Status != "succeeded" {
		return nil, errcode.ErrStateInvalid.WithMsg("视频未生成成功(status != succeeded),无法发布")
	}

	// 重复发布检查:已存在 status=on 的发布记录则拒绝
	if exist, gerr := s.r.Publish.GetByVideoID(ctx, in.FullVideoID); gerr == nil && exist != nil && exist.Status == "on" {
		return nil, errcode.ErrConflict.WithMsg("该视频已发布,请先下架后再发布")
	} else if gerr != nil && !errors.Is(gerr, gorm.ErrRecordNotFound) {
		return nil, gerr
	}

	// 可选:若存在 active 审核记录,需 approved
	if rec, rerr := s.r.Review.GetActiveRecord(ctx, "full_video", in.FullVideoID); rerr == nil && rec != nil {
		if rec.Status != "approved" {
			return nil, errcode.ErrStateInvalid.WithMsg("审核未通过,无法发布")
		}
	}

	p := &model.Publish{
		FullVideoID:     in.FullVideoID,
		PublishedBy:     uid,
		PublishedAt:     time.Now(),
		Status:          "on",
		WatermarkConfig: model.JSON(in.WatermarkConfig),
	}
	if err := s.r.Publish.Upsert(ctx, p); err != nil {
		return nil, err
	}
	return s.r.Publish.GetByVideoID(ctx, in.FullVideoID)
}

// Unpublish 下架某 FullVideoID 对应的发布记录(Status -> off,不做物理删除,保留历史计数)。
func (s *PublishService) Unpublish(ctx context.Context, videoID int64) error {
	p, err := s.r.Publish.GetByVideoID(ctx, videoID)
	if err != nil || p == nil {
		return errcode.ErrNotFound
	}
	if p.Status == "off" {
		return errcode.ErrStateInvalid.WithMsg("该视频已是下架状态")
	}
	return s.r.Publish.SetStatus(ctx, p.ID, "off")
}

// Get 查询某 FullVideoID 的发布记录;不存在时返回 ErrNotFound。
func (s *PublishService) Get(ctx context.Context, videoID int64) (*model.Publish, error) {
	p, err := s.r.Publish.GetByVideoID(ctx, videoID)
	if err != nil || p == nil {
		return nil, errcode.ErrNotFound
	}
	return p, nil
}

// List 列出发布记录(可按 status 过滤,分页)。
// status 仅接受 "on" / "off" / "" (全部);其他值视为参数错误。
func (s *PublishService) List(ctx context.Context, status string, page, size int) ([]model.Publish, int64, error) {
	if status != "" && status != "on" && status != "off" {
		return nil, 0, errcode.ErrParam.WithMsg("status 仅支持 on/off")
	}
	return s.r.Publish.List(ctx, status, page, size)
}

// IncPlay 播放计数 +1(原子自增,不做先读再写)。
func (s *PublishService) IncPlay(ctx context.Context, videoID int64) error {
	p, err := s.r.Publish.GetByVideoID(ctx, videoID)
	if err != nil || p == nil {
		return errcode.ErrNotFound
	}
	if p.Status != "on" {
		return errcode.ErrStateInvalid.WithMsg("视频已下架,不再统计播放")
	}
	return s.r.Publish.IncPlayCount(ctx, p.ID)
}

// IncDownload 下载计数 +1(原子自增)。
func (s *PublishService) IncDownload(ctx context.Context, videoID int64) error {
	p, err := s.r.Publish.GetByVideoID(ctx, videoID)
	if err != nil || p == nil {
		return errcode.ErrNotFound
	}
	if p.Status != "on" {
		return errcode.ErrStateInvalid.WithMsg("视频已下架,不再统计下载")
	}
	return s.r.Publish.IncDownloadCount(ctx, p.ID)
}

// UpdateWatermark 更新水印配置(JSON object)。
// 仅接受合法 JSON object,空 / 数组 / 字面量 / null 一律拒绝。
func (s *PublishService) UpdateWatermark(ctx context.Context, videoID int64, raw json.RawMessage) (*model.Publish, error) {
	if len(raw) == 0 {
		return nil, errcode.ErrParam.WithMsg("watermark_config 不能为空")
	}
	if err := validateJSONObject(raw); err != nil {
		return nil, err
	}
	p, err := s.r.Publish.GetByVideoID(ctx, videoID)
	if err != nil || p == nil {
		return nil, errcode.ErrNotFound
	}
	p.WatermarkConfig = model.JSON(raw)
	if err := s.r.Publish.Upsert(ctx, p); err != nil {
		return nil, err
	}
	return s.r.Publish.GetByVideoID(ctx, videoID)
}
