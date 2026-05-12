package service

import (
	"context"
	"testing"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"go.uber.org/zap"
)

func newBillingService(t *testing.T) (*BillingService, *repo.Repositories) {
	db := newTestDB(t, &model.BillingQuota{}, &model.BillingDaily{})
	r := newTestRepos(db)
	return &BillingService{r: r, log: zap.NewNop()}, r
}

// TestBillingService_RollupAggregate 覆盖 Rollup 的日聚合行为:
//   - 同一天 + 相同维度的多次写入,calls/tokens/cost 累加
//   - 不同天写入,产生独立行
//   - StatDate 自动截到 0:00
//   - 单条 invocation 也能写出独立日报
func TestBillingService_RollupAggregate(t *testing.T) {
	ctx := context.Background()
	s, r := newBillingService(t)

	loc := time.UTC
	day1 := time.Date(2026, 5, 12, 10, 30, 0, 0, loc)
	day1Other := time.Date(2026, 5, 12, 23, 59, 59, 0, loc) // 同一天另一时刻
	day2 := time.Date(2026, 5, 13, 0, 1, 0, 0, loc)

	rolls := []RollupParams{
		{Date: day1, ModelID: 1, DeptID: 10, UserID: 100, Calls: 1, InputTok: 100, OutputTok: 50, Units: 1, Cost: 0.10},
		{Date: day1Other, ModelID: 1, DeptID: 10, UserID: 100, Calls: 2, InputTok: 200, OutputTok: 60, Units: 1, Cost: 0.20},
		{Date: day2, ModelID: 1, DeptID: 10, UserID: 100, Calls: 5, InputTok: 500, OutputTok: 100, Units: 3, Cost: 1.00},
		// 不同 user/dept 也应单独写一行
		{Date: day1, ModelID: 1, DeptID: 11, UserID: 101, Calls: 7, InputTok: 70, OutputTok: 7, Units: 7, Cost: 0.07},
	}
	for i := range rolls {
		if err := s.Rollup(ctx, &rolls[i]); err != nil {
			t.Fatalf("rollup #%d: %v", i, err)
		}
	}

	// 取 2026-05-12 的聚合
	from := time.Date(2026, 5, 12, 0, 0, 0, 0, loc)
	to := time.Date(2026, 5, 12, 23, 59, 59, 0, loc)
	list, err := s.ListDaily(ctx, from, to, 100, 10, 1)
	if err != nil {
		t.Fatalf("list daily: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expect 1 aggregated row for (uid=100,dept=10) day1, got %d", len(list))
	}
	row := list[0]
	if row.Calls != 3 {
		t.Errorf("calls aggregated = %d want 3", row.Calls)
	}
	if row.InputTokens != 300 {
		t.Errorf("input_tokens = %d want 300", row.InputTokens)
	}
	if row.OutputTokens != 110 {
		t.Errorf("output_tokens = %d want 110", row.OutputTokens)
	}
	if row.Cost < 0.299 || row.Cost > 0.301 {
		t.Errorf("cost = %v want ~0.30", row.Cost)
	}
	if !row.StatDate.Equal(from) {
		t.Errorf("StatDate not truncated to 00:00, got %v", row.StatDate)
	}

	// 跨 2 天范围,应该 day1 + day2 都有 user=100 那条
	to2 := time.Date(2026, 5, 13, 23, 59, 59, 0, loc)
	all, err := s.ListDaily(ctx, from, to2, 100, 10, 1)
	if err != nil {
		t.Fatalf("list daily 2d: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expect 2 rows across 2 days, got %d", len(all))
	}

	// 全部维度不过滤,日 1 应该有 user=100 与 user=101 两条
	day1All, err := s.ListDaily(ctx, from, to, 0, 0, 0)
	if err != nil {
		t.Fatalf("list day1 no filter: %v", err)
	}
	if len(day1All) != 2 {
		t.Fatalf("day1 with no filter expect 2 rows, got %d", len(day1All))
	}

	_ = r // keep for debug
}

// TestBillingService_RollupDeductsQuota_ScopeSwitch 覆盖 scope_type 切换的扣减逻辑。
// FindActive 在 user 与 dept 同时命中时优先 user;指定 model > model=0。
func TestBillingService_RollupDeductsQuota_ScopeSwitch(t *testing.T) {
	ctx := context.Background()
	s, r := newBillingService(t)

	// 创建 4 条配额:
	// q1: user=100, model=0, metric=calls -> 通用 user 配额
	// q2: dept=10, model=0, metric=calls -> 通用 dept 配额
	// q3: user=100, model=1, metric=calls -> 指定 model 的 user 配额(优先级最高)
	// q4: dept=10, model=0, metric=cost   -> metric 不同,不应被扣
	mustCreateQuota(t, r, &model.BillingQuota{ScopeType: "user", ScopeID: 100, ModelID: 0, Metric: "calls", QuotaValue: 100, Enabled: 1})
	mustCreateQuota(t, r, &model.BillingQuota{ScopeType: "dept", ScopeID: 10, ModelID: 0, Metric: "calls", QuotaValue: 100, Enabled: 1})
	q3 := &model.BillingQuota{ScopeType: "user", ScopeID: 100, ModelID: 1, Metric: "calls", QuotaValue: 100, Enabled: 1}
	mustCreateQuota(t, r, q3)
	q4 := &model.BillingQuota{ScopeType: "dept", ScopeID: 10, ModelID: 0, Metric: "cost", QuotaValue: 100, Enabled: 1}
	mustCreateQuota(t, r, q4)

	// 第 1 次 Rollup: user=100 + model=1 + metric=calls, usage=1
	// 期望: q3 被扣 1
	if err := s.Rollup(ctx, &RollupParams{
		Date: time.Now(), ModelID: 1, DeptID: 10, UserID: 100,
		Calls: 1, Metric: "calls", UsageValue: 1,
	}); err != nil {
		t.Fatalf("rollup: %v", err)
	}
	got, err := r.Billing.GetQuota(ctx, q3.ID)
	if err != nil {
		t.Fatalf("get q3: %v", err)
	}
	if got.UsedValue != 1 {
		t.Fatalf("q3 used = %v want 1 (best-match should win)", got.UsedValue)
	}

	// metric=cost 没被扣
	got4, _ := r.Billing.GetQuota(ctx, q4.ID)
	if got4.UsedValue != 0 {
		t.Errorf("q4 used = %v want 0 (different metric)", got4.UsedValue)
	}

	// metric 为空时只写日报不扣 quota
	if err := s.Rollup(ctx, &RollupParams{
		Date: time.Now(), ModelID: 1, DeptID: 10, UserID: 100,
		Calls: 1, Metric: "", UsageValue: 999,
	}); err != nil {
		t.Fatalf("rollup empty metric: %v", err)
	}
	got, _ = r.Billing.GetQuota(ctx, q3.ID)
	if got.UsedValue != 1 {
		t.Fatalf("q3 used should still be 1 when metric empty, got %v", got.UsedValue)
	}
}

// TestBillingService_RollupNilSafe nil receiver/nil params 不应 panic。
func TestBillingService_RollupNilSafe(t *testing.T) {
	var nilSvc *BillingService
	if err := nilSvc.Rollup(context.Background(), &RollupParams{Date: time.Now()}); err != nil {
		t.Errorf("nil svc rollup: %v", err)
	}
	s, _ := newBillingService(t)
	if err := s.Rollup(context.Background(), nil); err != nil {
		t.Errorf("nil params rollup: %v", err)
	}
}

// TestBillingService_CreateQuotaDefaults 验证创建配额时 period/enabled 默认值。
func TestBillingService_CreateQuotaDefaults(t *testing.T) {
	ctx := context.Background()
	s, _ := newBillingService(t)

	cases := []struct {
		name        string
		in          *CreateQuotaInput
		wantPeriod  string
		wantEnabled int8
	}{
		{"default monthly+enabled", &CreateQuotaInput{ScopeType: "user", ScopeID: 1, Metric: "calls", QuotaValue: 10}, "monthly", 1},
		{"explicit period kept", &CreateQuotaInput{ScopeType: "user", ScopeID: 2, Period: "daily", Metric: "tokens", QuotaValue: 1}, "daily", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q, err := s.CreateQuota(ctx, c.in)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if q.Period != c.wantPeriod {
				t.Errorf("period=%s want %s", q.Period, c.wantPeriod)
			}
			if q.Enabled != c.wantEnabled {
				t.Errorf("enabled=%d want %d", q.Enabled, c.wantEnabled)
			}
		})
	}
}

func mustCreateQuota(t *testing.T, r *repo.Repositories, q *model.BillingQuota) {
	t.Helper()
	if err := r.Billing.CreateQuota(context.Background(), q); err != nil {
		t.Fatalf("create quota: %v", err)
	}
}
