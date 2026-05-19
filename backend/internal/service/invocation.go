package service

import (
	"context"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

// invocationService handles model invocation logging and queries.
type invocationService struct {
	r   *repo.Repositories
	log *zap.Logger
}

// LogParams captures one invocation event.
type LogParams struct {
	ModelID      int64
	UserID       int64
	DeptID       int64
	ProjectID    int64
	BizType      string
	BizRef       string
	InputTokens  int
	OutputTokens int
	Units        int
	DurationMs   int
	Cost         float64
	Status       string
	ErrorCode    string
	StartedAt    time.Time
	EndedAt      *time.Time
}

// Log writes one invocation record and never blocks the main flow on failure.
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

// List returns invocation records.
func (s *invocationService) List(ctx context.Context, q *repo.ListInvocationsQuery) ([]model.ModelInvocation, int64, error) {
	list, total, err := s.r.Invocation.List(ctx, q)
	if err != nil {
		return nil, 0, errcode.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

// Stats returns aggregated invocation metrics.
func (s *invocationService) Stats(ctx context.Context, q *repo.ListInvocationsQuery) (*repo.InvocationStats, error) {
	stats, err := s.r.Invocation.StatsAll(ctx, q)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return stats, nil
}
