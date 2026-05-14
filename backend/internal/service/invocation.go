// InvocationService 调用日志(model_invocations)聚合与查询。
package service

import (
	"context"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

type invocationService struct {
	r   *repo.Repositories
	log *zap.Logger
}

// LogParams 写入调用日志的入参,用于 worker / handler 在调用模型成功或失败后记录
type LogParams struct {
	ModelID      int64
	UserID       int64
	DeptID       int64
	ProjectID    int64
	BizType      string // script_split / prompt_gen / image_gen / video_gen / tts ...
	BizRef       string // 如 episode:123 / image:42
	InputTokens  int
	OutputTokens int
	Units        int // 图像张数 / 视频秒数 等
	DurationMs   int
	Cost         float64
	Status       string // succeeded / failed
	ErrorCode    string
	StartedAt    time.Time
	EndedAt      *time.Time
}

// Log 写入一条调用日志。任何错误只记录日志,不阻塞主流程。
func (s *invocationService) Log(ctx context.Context, p *LogParams) {
	if s == nil || s.r == nil {
		return
	}
	if p.Status == "" {
		p.Status = "succeeded"
	}
	if p.StartedAt.IsZero() {
		p.StartedAt = time.Now()
	}
	inv := &model.ModelInvocation{
		ModelID:      p.ModelID,
		UserID:       p.UserID,
		DeptID:       p.DeptID,
		ProjectID:    p.ProjectID,
		BizType:      p.BizType,
		BizRef:       p.BizRef,
		InputTokens:  p.InputTokens,
		OutputTokens: p.OutputTokens,
		Units:        p.Units,
		DurationMs:   p.DurationMs,
		Cost:         p.Cost,
		Status:       p.Status,
		ErrorCode:    p.ErrorCode,
		StartedAt:    p.StartedAt,
		EndedAt:      p.EndedAt,
	}
	if err := s.r.Invocation.Create(ctx, inv); err != nil && s.log != nil {
		s.log.Warn("write invocation log", zap.Error(err))
	}
}

// List 调用日志分页列表
func (s *invocationService) List(ctx context.Context, q *repo.ListInvocationsQuery) ([]model.ModelInvocation, int64, error) {
	list, total, err := s.r.Invocation.List(ctx, q)
	if err != nil {
		return nil, 0, errcode.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

// Stats 调用日志聚合统计
func (s *invocationService) Stats(ctx context.Context, q *repo.ListInvocationsQuery) (*repo.InvocationStats, error) {
	stats, err := s.r.Invocation.StatsAll(ctx, q)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return stats, nil
}
