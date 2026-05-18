package repo

import (
	"context"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type AuditRepo struct{ db *gorm.DB }

func (r *AuditRepo) Create(ctx context.Context, log *model.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

type ListAuditQuery struct {
	Page, PageSize int
	UserID         int64
	ResourceType   string
	Action         string
}

func (r *AuditRepo) List(ctx context.Context, q *ListAuditQuery) ([]model.AuditLog, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.AuditLog{})
	if q.UserID != 0 {
		tx = tx.Where("user_id = ?", q.UserID)
	}
	if q.ResourceType != "" {
		tx = tx.Where("resource_type = ?", q.ResourceType)
	}
	if q.Action != "" {
		tx = tx.Where("action = ?", q.Action)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.AuditLog
	if err := tx.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
