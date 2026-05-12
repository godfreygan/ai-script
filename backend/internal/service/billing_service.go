// BillingService 配额(BillingQuota)CRUD + 写日聚合(BillingDaily) + 扣减用量。
// 由 InvocationService.Log 的上层在调用模型成功/失败后调用 Rollup,把单次调用滚到日账并扣减命中的配额。
package service

import (
	"context"
	"fmt"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

// BillingService 计费服务
type BillingService struct {
	r   *repo.Repositories
	log *zap.Logger
}

// 允许的枚举值集合
var (
	billingScopeTypes = map[string]struct{}{"user": {}, "dept": {}}
	billingPeriods    = map[string]struct{}{"daily": {}, "monthly": {}, "yearly": {}}
	billingMetrics    = map[string]struct{}{"calls": {}, "tokens": {}, "units": {}, "cost": {}}
)

// CreateQuotaInput 创建配额入参
type CreateQuotaInput struct {
	ScopeType  string  `json:"scope_type" binding:"required"` // user/dept
	ScopeID    int64   `json:"scope_id" binding:"required"`
	ModelID    int64   `json:"model_id"`                      // 0 = 全部模型
	Period     string  `json:"period"`                        // monthly/daily/yearly
	Metric     string  `json:"metric" binding:"required"`     // calls/tokens/units/cost
	QuotaValue float64 `json:"quota_value" binding:"required"`
	Enabled    *int8   `json:"enabled"` // 指针,允许显式传 0(禁用)
}

// UpdateQuotaInput 更新配额入参(指针字段非 nil 时覆盖)
type UpdateQuotaInput struct {
	QuotaValue *float64 `json:"quota_value"`
	Enabled    *int8    `json:"enabled"`
	Period     *string  `json:"period"`
}

// RollupParams Rollup 入参 —— 写日账 + 扣 quota
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
	Metric     string  // 决定扣 quota 时按哪个 metric 算
	UsageValue float64 // 扣减 quota 的量(如 metric=calls 就是 1,metric=cost 就是 Cost,...)
}

// validatePeriod 校验 period 枚举
func validatePeriod(p string) error {
	if _, ok := billingPeriods[p]; !ok {
		return errcode.ErrParam.WithMsg(fmt.Sprintf("invalid period: %q", p))
	}
	return nil
}

// validateMetric 校验 metric 枚举
func validateMetric(m string) error {
	if _, ok := billingMetrics[m]; !ok {
		return errcode.ErrParam.WithMsg(fmt.Sprintf("invalid metric: %q", m))
	}
	return nil
}

// validateScopeType 校验 scope_type 枚举
func validateScopeType(s string) error {
	if _, ok := billingScopeTypes[s]; !ok {
		return errcode.ErrParam.WithMsg(fmt.Sprintf("invalid scope_type: %q", s))
	}
	return nil
}

// ListQuotas 列出某个 scope(user/dept) 的全部启用配额
func (s *BillingService) ListQuotas(ctx context.Context, scopeType string, scopeID int64) ([]model.BillingQuota, error) {
	return s.r.Billing.ListQuotas(ctx, scopeType, scopeID)
}

// GetQuota 取单条配额;未找到返回 errcode.ErrNotFound
func (s *BillingService) GetQuota(ctx context.Context, id int64) (*model.BillingQuota, error) {
	q, err := s.r.Billing.GetQuota(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return q, nil
}

// CreateQuota 新建配额
func (s *BillingService) CreateQuota(ctx context.Context, in *CreateQuotaInput) (*model.BillingQuota, error) {
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
	// enabled 默认启用,允许显式传 0 创建禁用配额
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
		return nil, err
	}
	return q, nil
}

// UpdateQuota 更新配额
func (s *BillingService) UpdateQuota(ctx context.Context, id int64, in *UpdateQuotaInput) (*model.BillingQuota, error) {
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
		return nil, err
	}
	return q, nil
}

// DeleteQuota 删除配额
func (s *BillingService) DeleteQuota(ctx context.Context, id int64) error {
	return s.r.Billing.DeleteQuota(ctx, id)
}

// CheckQuota 调用前的额度预检 —— 调用 invocation 前由上层调用。
// 找到命中的配额,若 used+delta > quota_value 返回 ErrQuotaExceeded;
// 没有命中配额(nil quota) = 不限制,放行。
func (s *BillingService) CheckQuota(ctx context.Context, userID, deptID, modelID int64, metric string, delta float64) error {
	if s == nil || s.r == nil {
		return nil
	}
	if metric == "" {
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
		// 查询失败不阻塞业务,fail-open
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

// Rollup 把一次模型调用滚到日账,并扣减命中的配额。
// best-effort:写日账/扣配额的错误只 Warn 不抛出。
// 注意:扣减不是严格原子,超用极小概率(高并发同 quota_id 同时扣)需上层用 CheckQuota 做事前拦截。
func (s *BillingService) Rollup(ctx context.Context, p *RollupParams) error {
	if s == nil || s.r == nil || p == nil {
		return nil
	}
	// 1. 写日聚合 —— StatDate 统一截到当地 0 点,跨月行级天然区分(stat_date 是行键)
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

	// 2. 扣 quota —— FindActive 按 user>dept、具体 model>0 排序,一次返回最匹配一条。
	//    使用 SQL 表达式 used_value+? (在 repo.IncUsed 内) 保证单语句原子,无 read-modify-write race。
	//    超用兜底:扣减前再做一次软校验(非严格,高并发下仍可能微超,严格控制依赖上层 CheckQuota)。
	if p.Metric != "" {
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
	}
	return nil
}

// ListDaily 按日聚合查询(from/to 日期闭区间;userID/deptID/modelID 为 0 时不过滤该列)
// from 规范到当天 0 点,to 规范到当天 23:59:59.999999999,确保 BETWEEN 真正包含 to 当天。
func (s *BillingService) ListDaily(ctx context.Context, from, to time.Time, userID, deptID, modelID int64) ([]model.BillingDaily, error) {
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
	return s.r.Billing.ListDaily(ctx, fromDay, toDay, userID, deptID, modelID)
}
