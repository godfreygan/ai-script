package repo

import (
	"context"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	// 修复 P0 #7 — 原 Save(q) 把零值字段也写回(并发 patch 会丢更新)。改用 Updates(map)
	// 只写显式字段。used_value 用 IncUsed 独立通道,这里不动。
	return r.db.WithContext(ctx).Model(&model.BillingQuota{}).Where("id = ?", q.ID).
		Updates(map[string]any{
			"scope_type":  q.ScopeType,
			"scope_id":    q.ScopeID,
			"model_id":    q.ModelID,
			"period":      q.Period,
			"metric":      q.Metric,
			"quota_value": q.QuotaValue,
			"reset_at":    q.ResetAt,
			"enabled":     q.Enabled,
		}).Error
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
//
// 修复 P0 #5 — 原实现是 Transaction(First→Updates|Create),两步操作在并发下:
//   - T1 First 返回 not found, T1 还没 Create
//   - T2 First 也返回 not found, T2 Create 成功
//   - T1 再次 Create → 主键/唯一冲突 → 整个事务回滚 → 当天的 calls/cost 数据被吞掉
//
// 改用 INSERT ... ON DUPLICATE KEY UPDATE(MySQL 原生原子语义),
// DB 行锁 + (stat_date,model_id,dept_id,user_id) 唯一索引兜底,
// 让 MySQL 做加法,Go 这边不再有 race window。
//
// 唯一索引在 model.BillingDaily 已用 gorm:"uniqueIndex:uniq_daily_dim,priority:N" 声明,
// AutoMigrate 启动时自动创建,无需手工 SQL。
func (r *BillingRepo) UpsertDaily(ctx context.Context, d *model.BillingDaily) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "stat_date"},
			{Name: "model_id"},
			{Name: "dept_id"},
			{Name: "user_id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"calls":         gorm.Expr("calls + ?", d.Calls),
			"input_tokens":  gorm.Expr("input_tokens + ?", d.InputTokens),
			"output_tokens": gorm.Expr("output_tokens + ?", d.OutputTokens),
			"units":         gorm.Expr("units + ?", d.Units),
			"cost":          gorm.Expr("cost + ?", d.Cost),
			"updated_at":    time.Now(),
		}),
	}).Create(d).Error
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
