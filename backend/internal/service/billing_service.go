package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// billingService manages quota CRUD and daily usage rollups.
type billingService struct {
	r   *repo.Repositories
	log *zap.Logger
}

var (
	billingScopeTypes = map[string]struct{}{"user": {}, "dept": {}}
	billingPeriods    = map[string]struct{}{"daily": {}, "monthly": {}, "yearly": {}}
	billingMetrics    = map[string]struct{}{"calls": {}, "tokens": {}, "units": {}, "cost": {}}
)

// CreateQuotaInput creates one quota record.
type CreateQuotaInput struct {
	ScopeType  string  `json:"scope_type" binding:"required,oneof=user dept"`
	ScopeID    int64   `json:"scope_id" binding:"required,gte=1"`
	ModelID    int64   `json:"model_id" binding:"gte=0"`
	Period     string  `json:"period" binding:"omitempty,oneof=daily monthly yearly"`
	Metric     string  `json:"metric" binding:"required,oneof=calls tokens units cost"`
	QuotaValue float64 `json:"quota_value" binding:"required,gte=0"`
	Enabled    *int8   `json:"enabled" binding:"omitempty,gte=0,lte=1"`
}

// UpdateQuotaInput updates mutable quota fields.
type UpdateQuotaInput struct {
	QuotaValue *float64 `json:"quota_value" binding:"omitempty,gte=0"`
	Enabled    *int8    `json:"enabled" binding:"omitempty,gte=0,lte=1"`
	Period     *string  `json:"period" binding:"omitempty,oneof=daily monthly yearly"`
}

// RollupParams describes one usage rollup event.
type RollupParams struct {
	Date       time.Time
	ModelID    int64
	DeptID     int64
	UserID     int64
	Calls      int
	InputTok   int64
	OutputTok  int64
	Units      int64
	Cost       float64
	Metric     string
	UsageValue float64
}

func validatePeriod(p string) error {
	if _, ok := billingPeriods[p]; !ok {
		return errcode.ErrParam.WithMsg(fmt.Sprintf("invalid period: %q", p))
	}
	return nil
}

func validateMetric(m string) error {
	if _, ok := billingMetrics[m]; !ok {
		return errcode.ErrParam.WithMsg(fmt.Sprintf("invalid metric: %q", m))
	}
	return nil
}

func validateScopeType(s string) error {
	if _, ok := billingScopeTypes[s]; !ok {
		return errcode.ErrParam.WithMsg(fmt.Sprintf("invalid scope_type: %q", s))
	}
	return nil
}

// ListQuotas returns active quotas for one scope.
func (s *billingService) ListQuotas(ctx context.Context, scopeType string, scopeID int64) ([]model.BillingQuota, error) {
	return s.r.Billing.ListQuotas(ctx, scopeType, scopeID)
}

// GetQuota returns one quota by id.
func (s *billingService) GetQuota(ctx context.Context, id int64) (*model.BillingQuota, error) {
	q, err := s.r.Billing.GetQuota(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return q, nil
}

// CreateQuota inserts a new quota.
func (s *billingService) CreateQuota(ctx context.Context, in *CreateQuotaInput) (*model.BillingQuota, error) {
	if in == nil {
		return nil, errcode.ErrParam.WithMsg("nil input")
	}
	if err := validateScopeType(in.ScopeType); err != nil {
		return nil, err
	}
	if in.ScopeID <= 0 {
		return nil, errcode.ErrParam.WithMsg("scope_id must be > 0")
	}
	if in.ModelID < 0 {
		return nil, errcode.ErrParam.WithMsg("model_id must be >= 0")
	}
	if err := validateMetric(in.Metric); err != nil {
		return nil, err
	}
	if in.QuotaValue < 0 {
		return nil, errcode.ErrParam.WithMsg("quota_value must be >= 0")
	}
	period := in.Period
	if period == "" {
		period = "monthly"
	}
	if err := validatePeriod(period); err != nil {
		return nil, err
	}

	var enabled int8 = 1
	if in.Enabled != nil {
		enabled = *in.Enabled
		if enabled != 0 && enabled != 1 {
			return nil, errcode.ErrParam.WithMsg("enabled must be 0 or 1")
		}
	}

	q := &model.BillingQuota{
		ScopeType:  in.ScopeType,
		ScopeID:    in.ScopeID,
		ModelID:    in.ModelID,
		Period:     period,
		Metric:     in.Metric,
		QuotaValue: in.QuotaValue,
		Enabled:    enabled,
	}
	if err := s.r.Billing.CreateQuota(ctx, q); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return q, nil
}

// UpdateQuota updates one quota.
func (s *billingService) UpdateQuota(ctx context.Context, id int64, in *UpdateQuotaInput) (*model.BillingQuota, error) {
	if in == nil {
		return nil, errcode.ErrParam.WithMsg("nil input")
	}
	q, err := s.GetQuota(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.QuotaValue != nil {
		if *in.QuotaValue < 0 {
			return nil, errcode.ErrParam.WithMsg("quota_value must be >= 0")
		}
		q.QuotaValue = *in.QuotaValue
	}
	if in.Enabled != nil {
		if *in.Enabled != 0 && *in.Enabled != 1 {
			return nil, errcode.ErrParam.WithMsg("enabled must be 0 or 1")
		}
		q.Enabled = *in.Enabled
	}
	if in.Period != nil {
		if err := validatePeriod(*in.Period); err != nil {
			return nil, err
		}
		q.Period = *in.Period
	}
	if err := s.r.Billing.UpdateQuota(ctx, q); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return q, nil
}

// DeleteQuota removes one quota.
func (s *billingService) DeleteQuota(ctx context.Context, id int64) error {
	if err := s.r.Billing.DeleteQuota(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrNotFound
		}
		return errcode.ErrInternal.Wrap(err)
	}
	return nil
}

// CheckQuota performs a best-effort quota precheck.
func (s *billingService) CheckQuota(ctx context.Context, userID, deptID, modelID int64, metric string, delta float64) error {
	if s == nil || s.r == nil || metric == "" {
		return nil
	}
	if err := validateMetric(metric); err != nil {
		return err
	}
	quota, err := s.r.Billing.FindActive(ctx, userID, deptID, modelID, metric)
	if err != nil {
		if s.log != nil {
			s.log.Warn("billing check find active", zap.Error(err))
		}
		return nil
	}
	if quota == nil {
		return nil
	}
	if quota.UsedValue+delta > quota.QuotaValue {
		return errcode.ErrQuotaExceeded
	}
	return nil
}

// Rollup writes daily usage and increments the matched quota if present.
func (s *billingService) Rollup(ctx context.Context, p *RollupParams) error {
	if s == nil || s.r == nil || p == nil {
		return nil
	}

	date := p.Date
	if date.IsZero() {
		date = time.Now()
	}
	loc := date.Location()
	if loc == nil {
		loc = time.Local
	}
	statDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	daily := &model.BillingDaily{
		StatDate:     statDate,
		ModelID:      p.ModelID,
		DeptID:       p.DeptID,
		UserID:       p.UserID,
		Calls:        p.Calls,
		InputTokens:  p.InputTok,
		OutputTokens: p.OutputTok,
		Units:        p.Units,
		Cost:         p.Cost,
	}
	if err := s.r.Billing.UpsertDaily(ctx, daily); err != nil && s.log != nil {
		s.log.Warn("billing upsert daily", zap.Error(err))
	}

	if p.Metric == "" {
		return nil
	}
	if _, ok := billingMetrics[p.Metric]; !ok {
		if s.log != nil {
			s.log.Warn("billing rollup unknown metric", zap.String("metric", p.Metric))
		}
		return nil
	}

	quota, err := s.r.Billing.FindActive(ctx, p.UserID, p.DeptID, p.ModelID, p.Metric)
	if err != nil {
		if s.log != nil {
			s.log.Warn("billing find active quota", zap.Error(err))
		}
		return nil
	}
	if quota != nil {
		if err := s.r.Billing.IncUsed(ctx, quota.ID, p.UsageValue); err != nil && s.log != nil {
			s.log.Warn("billing inc used", zap.Error(err), zap.Int64("quota_id", quota.ID))
		}
	}
	return nil
}

// ListDaily returns daily usage between from and to, inclusive.
func (s *billingService) ListDaily(ctx context.Context, from, to time.Time, userID, deptID, modelID int64) ([]model.BillingDaily, error) {
	if from.IsZero() || to.IsZero() {
		return nil, errcode.ErrParam.WithMsg("from/to required")
	}
	if to.Before(from) {
		return nil, errcode.ErrParam.WithMsg("to must be >= from")
	}
	loc := from.Location()
	if loc == nil {
		loc = time.Local
	}
	fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	toDay := time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), loc)
	list, err := s.r.Billing.ListDaily(ctx, fromDay, toDay, userID, deptID, modelID)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return list, nil
}
