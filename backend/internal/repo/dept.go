package repo

import (
	"context"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type DeptRepo struct {
	db  *gorm.DB
	rdb *redis.Client
}

func (r *DeptRepo) WithDB(db *gorm.DB) *DeptRepo {
	return &DeptRepo{db: db, rdb: r.rdb}
}

func (r *DeptRepo) WithRedis(rdb *redis.Client) *DeptRepo {
	return &DeptRepo{db: r.db, rdb: rdb}
}

func (r *DeptRepo) List(ctx context.Context) ([]model.Department, error) {
	loader := func(ctx context.Context) ([]model.Department, error) {
		var list []model.Department
		if err := r.db.WithContext(ctx).Order("sort, id").Find(&list).Error; err != nil {
			return nil, err
		}
		return list, nil
	}
	return Get(ctx, r.rdb, cacheKey("dept", "list"), loader, 5*time.Minute)
}

func (r *DeptRepo) Get(ctx context.Context, id int64) (*model.Department, error) {
	var d model.Department
	if err := r.db.WithContext(ctx).First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DeptRepo) Create(ctx context.Context, d *model.Department) error {
	if err := r.db.WithContext(ctx).Create(d).Error; err != nil {
		return err
	}
	Delete(ctx, r.rdb, cacheKey("dept", "list"))
	return nil
}

func (r *DeptRepo) Update(ctx context.Context, d *model.Department) error {
	if err := r.db.WithContext(ctx).Model(&model.Department{}).Select("*").Omit("created_at").Where("id = ?", d.ID).Updates(d).Error; err != nil {
		return err
	}
	Delete(ctx, r.rdb, cacheKey("dept", "list"))
	return nil
}

func (r *DeptRepo) Delete(ctx context.Context, id int64) error {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&model.User{}).Where("dept_id = ?", id).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return ErrDeptHasUsers
		}
		result := tx.Delete(&model.Department{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	Delete(ctx, r.rdb, cacheKey("dept", "list"))
	return nil
}
