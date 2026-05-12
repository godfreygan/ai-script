package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type DeptRepo struct{ db *gorm.DB }

func (r *DeptRepo) List(ctx context.Context) ([]model.Department, error) {
	var list []model.Department
	if err := r.db.WithContext(ctx).Order("sort, id").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *DeptRepo) Get(ctx context.Context, id int64) (*model.Department, error) {
	var d model.Department
	if err := r.db.WithContext(ctx).First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DeptRepo) Create(ctx context.Context, d *model.Department) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *DeptRepo) Update(ctx context.Context, d *model.Department) error {
	return r.db.WithContext(ctx).Save(d).Error
}

func (r *DeptRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&model.User{}).Where("dept_id = ?", id).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return ErrDeptHasUsers
		}
		return tx.Delete(&model.Department{}, id).Error
	})
}
