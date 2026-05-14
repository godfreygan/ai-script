package service

import (
	"context"
	"encoding/json"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

type auditService struct {
	r   *repo.Repositories
	log *zap.Logger
}

type LogAuditParams struct {
	UserID       int64
	Action       string
	ResourceType string
	ResourceID   string
	Before       any
	After        any
	IP           string
	UA           string
	RequestID    string
}

// Log records an audit entry best-effort. It never returns an error; failures are logged at warn level.
func (s *auditService) Log(ctx context.Context, p *LogAuditParams) {
	if p == nil {
		return
	}

	var beforeJSON model.JSON
	if p.Before != nil {
		b, err := json.Marshal(p.Before)
		if err != nil {
			s.log.Warn("audit: marshal before failed", zap.Error(err))
			beforeJSON = nil
		} else {
			beforeJSON = model.JSON(b)
		}
	}

	var afterJSON model.JSON
	if p.After != nil {
		b, err := json.Marshal(p.After)
		if err != nil {
			s.log.Warn("audit: marshal after failed", zap.Error(err))
			afterJSON = nil
		} else {
			afterJSON = model.JSON(b)
		}
	}

	entry := &model.AuditLog{
		UserID:       p.UserID,
		Action:       p.Action,
		ResourceType: p.ResourceType,
		ResourceID:   p.ResourceID,
		Before:       beforeJSON,
		After:        afterJSON,
		IP:           p.IP,
		UA:           p.UA,
		RequestID:    p.RequestID,
		CreatedAt:    time.Now(),
	}

	if err := s.r.Audit.Create(ctx, entry); err != nil {
		s.log.Warn("audit: create failed",
			zap.Error(err),
			zap.Int64("user_id", p.UserID),
			zap.String("action", p.Action),
			zap.String("resource_type", p.ResourceType),
			zap.String("resource_id", p.ResourceID),
			zap.String("request_id", p.RequestID),
		)
	}
}

// List returns audit log entries matching the query.
func (s *auditService) List(ctx context.Context, q *repo.ListAuditQuery) ([]model.AuditLog, int64, error) {
	list, total, err := s.r.Audit.List(ctx, q)
	if err != nil {
		return nil, 0, errcode.ErrInternal.Wrap(err)
	}
	return list, total, nil
}
