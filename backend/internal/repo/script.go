package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type ScriptRepo struct{ db *gorm.DB }

type ListScriptsQuery struct {
	Page, PageSize int
	ProjectID      int64
	Status         int8
	Q              string
}

func (r *ScriptRepo) List(ctx context.Context, q *ListScriptsQuery) ([]model.Script, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.Script{})
	if q.ProjectID != 0 {
		tx = tx.Where("project_id = ?", q.ProjectID)
	}
	if q.Status != 0 {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Q != "" {
		tx = tx.Where("name LIKE ?", "%"+q.Q+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.Script
	if err := tx.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *ScriptRepo) Get(ctx context.Context, id int64) (*model.Script, error) {
	var s model.Script
	if err := r.db.WithContext(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ScriptRepo) Create(ctx context.Context, s *model.Script) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *ScriptRepo) Update(ctx context.Context, s *model.Script) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *ScriptRepo) UpdateStatus(ctx context.Context, id int64, status int8) error {
	return r.db.WithContext(ctx).Model(&model.Script{}).Where("id = ?", id).Update("status", status).Error
}

func (r *ScriptRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Script{}, id).Error
}

// 版本
func (r *ScriptRepo) AddVersion(ctx context.Context, v *model.ScriptVersion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(v).Error; err != nil {
			return err
		}
		return tx.Model(&model.Script{}).Where("id = ?", v.ScriptID).
			Update("current_version", v.VersionNo).Error
	})
}

func (r *ScriptRepo) ListVersions(ctx context.Context, scriptID int64) ([]model.ScriptVersion, error) {
	var list []model.ScriptVersion
	err := r.db.WithContext(ctx).Where("script_id = ?", scriptID).
		Order("version_no desc").Find(&list).Error
	return list, err
}
