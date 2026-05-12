package repo

import (
	"context"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type ModelRepo struct{ db *gorm.DB }

type ListModelsQuery struct {
	Page, PageSize int
	Type           string
	Provider       string
	Enabled        int8
	Q              string
}

func (r *ModelRepo) List(ctx context.Context, q *ListModelsQuery) ([]model.Model, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.Model{})
	if q.Type != "" {
		tx = tx.Where("type = ?", q.Type)
	}
	if q.Provider != "" {
		tx = tx.Where("provider = ?", q.Provider)
	}
	if q.Enabled != 0 {
		tx = tx.Where("enabled = ?", q.Enabled)
	}
	if q.Q != "" {
		tx = tx.Where("name LIKE ? OR code LIKE ?", "%"+q.Q+"%", "%"+q.Q+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.Model
	if err := tx.Order("priority desc, id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *ModelRepo) ListAllEnabled(ctx context.Context) ([]model.Model, error) {
	var list []model.Model
	err := r.db.WithContext(ctx).Where("enabled = 1").Order("priority desc").Find(&list).Error
	return list, err
}

func (r *ModelRepo) Get(ctx context.Context, id int64) (*model.Model, error) {
	var m model.Model
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ModelRepo) GetByCode(ctx context.Context, code string) (*model.Model, error) {
	var m model.Model
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ModelRepo) Create(ctx context.Context, m *model.Model) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *ModelRepo) Update(ctx context.Context, m *model.Model) error {
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *ModelRepo) UpdateAPIKey(ctx context.Context, id int64, encrypted []byte) error {
	return r.db.WithContext(ctx).Model(&model.Model{}).
		Where("id = ?", id).Update("api_key_encrypted", encrypted).Error
}

func (r *ModelRepo) UpdateHealth(ctx context.Context, id int64, status int8) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Model{}).Where("id = ?", id).
		Updates(map[string]any{
			"last_health_at":     now,
			"last_health_status": status,
		}).Error
}

func (r *ModelRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Model{}, id).Error
}

// =============== ModelInvocation ===============

type InvocationRepo struct{ db *gorm.DB }

func (r *InvocationRepo) Create(ctx context.Context, inv *model.ModelInvocation) error {
	return r.db.WithContext(ctx).Create(inv).Error
}

type InvocationStats struct {
	Calls        int64   `json:"calls"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Units        int64   `json:"units"`
	Cost         float64 `json:"cost"`
}

func (r *InvocationRepo) StatsByUser(ctx context.Context, userID int64, from, to time.Time) (*InvocationStats, error) {
	var s InvocationStats
	err := r.db.WithContext(ctx).Model(&model.ModelInvocation{}).
		Select("COUNT(1) as calls, COALESCE(SUM(input_tokens),0) as input_tokens, COALESCE(SUM(output_tokens),0) as output_tokens, COALESCE(SUM(units),0) as units, COALESCE(SUM(cost),0) as cost").
		Where("user_id = ? AND started_at BETWEEN ? AND ?", userID, from, to).
		Scan(&s).Error
	return &s, err
}

type ListInvocationsQuery struct {
	Page, PageSize int
	UserID         int64
	DeptID         int64
	ProjectID      int64
	ModelID        int64
	BizType        string
	Status         string
	From           *time.Time
	To             *time.Time
}

func (r *InvocationRepo) List(ctx context.Context, q *ListInvocationsQuery) ([]model.ModelInvocation, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.ModelInvocation{})
	if q.UserID != 0 {
		tx = tx.Where("user_id = ?", q.UserID)
	}
	if q.DeptID != 0 {
		tx = tx.Where("dept_id = ?", q.DeptID)
	}
	if q.ProjectID != 0 {
		tx = tx.Where("project_id = ?", q.ProjectID)
	}
	if q.ModelID != 0 {
		tx = tx.Where("model_id = ?", q.ModelID)
	}
	if q.BizType != "" {
		tx = tx.Where("biz_type = ?", q.BizType)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.From != nil {
		tx = tx.Where("started_at >= ?", *q.From)
	}
	if q.To != nil {
		tx = tx.Where("started_at <= ?", *q.To)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.ModelInvocation
	if err := tx.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// StatsAll 全平台范围内的统计,可叠加过滤条件
func (r *InvocationRepo) StatsAll(ctx context.Context, q *ListInvocationsQuery) (*InvocationStats, error) {
	tx := r.db.WithContext(ctx).Model(&model.ModelInvocation{})
	if q.UserID != 0 {
		tx = tx.Where("user_id = ?", q.UserID)
	}
	if q.DeptID != 0 {
		tx = tx.Where("dept_id = ?", q.DeptID)
	}
	if q.ProjectID != 0 {
		tx = tx.Where("project_id = ?", q.ProjectID)
	}
	if q.ModelID != 0 {
		tx = tx.Where("model_id = ?", q.ModelID)
	}
	if q.BizType != "" {
		tx = tx.Where("biz_type = ?", q.BizType)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.From != nil {
		tx = tx.Where("started_at >= ?", *q.From)
	}
	if q.To != nil {
		tx = tx.Where("started_at <= ?", *q.To)
	}
	var s InvocationStats
	err := tx.Select("COUNT(1) as calls, COALESCE(SUM(input_tokens),0) as input_tokens, COALESCE(SUM(output_tokens),0) as output_tokens, COALESCE(SUM(units),0) as units, COALESCE(SUM(cost),0) as cost").
		Scan(&s).Error
	return &s, err
}

