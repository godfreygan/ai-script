package repo

import (
	"context"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type BillingRepo struct{ db *gorm.DB }

func (r *BillingRepo) ListQuotas(ctx context.Context, scopeType string, scopeID int64) ([]model.BillingQuota, error) {
	var list []model.BillingQuota
	tx := r.db.WithContext(ctx).Where("enabled = 1")
	if scopeType != "" {
		tx = tx.Where("scope_type = ?", scopeType)
	}
	if scopeID != 0 {
		tx = tx.Where("scope_id = ?", scopeID)
	}
	err := tx.Order("id desc").Find(&list).Error
	return list, err
}

func (r *BillingRepo) GetQuota(ctx context.Context, id int64) (*model.BillingQuota, error) {
	var q model.BillingQuota
	if err := r.db.WithContext(ctx).First(&q, id).Error; err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *BillingRepo) CreateQuota(ctx context.Context, q *model.BillingQuota) error {
	return r.db.WithContext(ctx).Create(q).Error
}

func (r *BillingRepo) UpdateQuota(ctx context.Context, q *model.BillingQuota) error {
	return r.db.WithContext(ctx).Save(q).Error
}

func (r *BillingRepo) DeleteQuota(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.BillingQuota{}, id).Error
}

// FindActive 找到一条命中的额度记录(优先匹配 user,再匹配 dept,再忽略 model)
func (r *BillingRepo) FindActive(ctx context.Context, userID, deptID, modelID int64, metric string) (*model.BillingQuota, error) {
	var list []model.BillingQuota
	err := r.db.WithContext(ctx).
		Where("enabled = 1 AND metric = ? AND (scope_type = 'user' AND scope_id = ? OR scope_type = 'dept' AND scope_id = ?) AND (model_id = 0 OR model_id = ?)",
			metric, userID, deptID, modelID).
		Find(&list).Error
	if err != nil || len(list) == 0 {
		return nil, err
	}
	// 选最匹配的:user > dept,具体 model > 0
	best := list[0]
	score := func(q *model.BillingQuota) int {
		s := 0
		if q.ScopeType == "user" {
			s += 2
		}
		if q.ModelID != 0 {
			s += 1
		}
		return s
	}
	for i := range list {
		if score(&list[i]) > score(&best) {
			best = list[i]
		}
	}
	return &best, nil
}

func (r *BillingRepo) IncUsed(ctx context.Context, id int64, delta float64) error {
	return r.db.WithContext(ctx).Model(&model.BillingQuota{}).Where("id = ?", id).
		UpdateColumn("used_value", gorm.Expr("used_value + ?", delta)).Error
}

// UpsertDaily 写入或累加日聚合
func (r *BillingRepo) UpsertDaily(ctx context.Context, d *model.BillingDaily) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exist model.BillingDaily
		err := tx.Where("stat_date = ? AND model_id = ? AND dept_id = ? AND user_id = ?",
			d.StatDate, d.ModelID, d.DeptID, d.UserID).First(&exist).Error
		if err == nil {
			return tx.Model(&exist).Updates(map[string]any{
				"calls":         exist.Calls + d.Calls,
				"input_tokens":  exist.InputTokens + d.InputTokens,
				"output_tokens": exist.OutputTokens + d.OutputTokens,
				"units":         exist.Units + d.Units,
				"cost":          exist.Cost + d.Cost,
			}).Error
		}
		return tx.Create(d).Error
	})
}

func (r *BillingRepo) ListDaily(ctx context.Context, from, to time.Time, userID, deptID, modelID int64) ([]model.BillingDaily, error) {
	tx := r.db.WithContext(ctx).Where("stat_date BETWEEN ? AND ?", from, to)
	if userID != 0 {
		tx = tx.Where("user_id = ?", userID)
	}
	if deptID != 0 {
		tx = tx.Where("dept_id = ?", deptID)
	}
	if modelID != 0 {
		tx = tx.Where("model_id = ?", modelID)
	}
	var list []model.BillingDaily
	err := tx.Order("stat_date asc").Find(&list).Error
	return list, err
}
