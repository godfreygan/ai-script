package service

import (
	"context"
	"encoding/json"
	"hash/crc32"
	"strconv"
	"testing"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB 启动一个 in-memory sqlite,AutoMigrate 出 Sprint5 涉及的表。
// 不直接调用 r.Migrate(), 因为它会拉所有表(有些 model 在 sqlite 下可能报错),只迁需要的那几张。
// 注意:每个测试都调用 newTestDB,这里用 `:memory:` + SetMaxOpenConns(1) 拿到独占的内存库,
// 避免 `cache=shared` 多测试共用同一个 DB 时再次 AutoMigrate 同名表触发 sqlite 方言 ALTER 报错。
func newTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// sqlite `:memory:` 默认每个 *sql.Conn 各持一个独立内存库,会出现 AutoMigrate 在 conn A
	// 建表、后续查询走 conn B 看不到表的现象。把池子限制为 1,保证 schema 全程可见。
	if sqlDB, derr := db.DB(); derr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// newTestRepos 构造一个最小可用的 Repositories(仅初始化测试用到的子 repo)。
func newTestRepos(db *gorm.DB) *repo.Repositories {
	return repo.NewRepositories(db, nil)
}

func newFeatureFlagService(t *testing.T) (*FeatureFlagService, *repo.Repositories) {
	db := newTestDB(t, &model.FeatureFlag{})
	r := newTestRepos(db)
	return &FeatureFlagService{r: r, log: zap.NewNop()}, r
}

// rolloutHit 复算 service 中的灰度算法,用于测试中精准选 user id。
func rolloutHit(key string, uid int64) int {
	return int(crc32.ChecksumIEEE([]byte(key+":"+strconv.FormatInt(uid, 10))) % 100)
}

func TestFeatureFlagService_Evaluate(t *testing.T) {
	ctx := context.Background()
	s, r := newFeatureFlagService(t)

	// 预置:多个 flag
	// 1) disabled flag 无论命中啥都不开
	mustCreateFlag(t, r, &model.FeatureFlag{Key: "off", Enabled: 0, Rollout: 100,
		Rules: model.JSON([]byte(`{"users":[1]}`))})
	// 2) user 命中
	mustCreateFlag(t, r, &model.FeatureFlag{Key: "user_hit", Enabled: 1, Rollout: 0,
		Rules: model.JSON([]byte(`{"users":[42],"depts":[],"projects":[]}`))})
	// 3) dept 命中
	mustCreateFlag(t, r, &model.FeatureFlag{Key: "dept_hit", Enabled: 1, Rollout: 0,
		Rules: model.JSON([]byte(`{"depts":[99]}`))})
	// 4) rollout 100
	mustCreateFlag(t, r, &model.FeatureFlag{Key: "all_on", Enabled: 1, Rollout: 100})
	// 5) rollout 0 + 无规则
	mustCreateFlag(t, r, &model.FeatureFlag{Key: "all_off", Enabled: 1, Rollout: 0})
	// 6) 畸形 JSON,fallback 到 rollout
	mustCreateFlag(t, r, &model.FeatureFlag{Key: "broken", Enabled: 1, Rollout: 100,
		Rules: model.JSON([]byte(`{not-json`))})
	// 7) rollout 50% —— 选两个 uid: 一个 hash<50,一个 hash>=50
	mustCreateFlag(t, r, &model.FeatureFlag{Key: "half", Enabled: 1, Rollout: 50})

	// 找 half rollout 的边界 uid
	var lo, hi int64 = -1, -1
	for u := int64(1); u <= 200 && (lo < 0 || hi < 0); u++ {
		h := rolloutHit("half", u)
		if h < 50 && lo < 0 {
			lo = u
		}
		if h >= 50 && hi < 0 {
			hi = u
		}
	}
	if lo < 0 || hi < 0 {
		t.Fatalf("failed to locate rollout boundary uids")
	}

	cases := []struct {
		name string
		key  string
		fc   *FlagContext
		want bool
	}{
		{"flag disabled returns false", "off", &FlagContext{UserID: 1}, false},
		{"missing flag returns false", "ghost", &FlagContext{UserID: 1}, false},
		{"user id hit", "user_hit", &FlagContext{UserID: 42}, true},
		{"user id miss falls through rollout zero", "user_hit", &FlagContext{UserID: 7}, false},
		{"dept id hit", "dept_hit", &FlagContext{DeptID: 99}, true},
		{"dept id miss", "dept_hit", &FlagContext{DeptID: 1}, false},
		{"rollout 100 all on", "all_on", &FlagContext{UserID: 1}, true},
		{"rollout 0 all off", "all_off", &FlagContext{UserID: 1}, false},
		{"malformed json falls back to rollout", "broken", &FlagContext{UserID: 1}, true},
		{"rollout 50 lo bucket", "half", &FlagContext{UserID: lo}, true},
		{"rollout 50 hi bucket", "half", &FlagContext{UserID: hi}, false},
		{"nil context with rollout 100", "all_on", nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.Evaluate(ctx, tc.key, tc.fc)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Evaluate(%q)=%v want %v", tc.key, got, tc.want)
			}
		})
	}
}

func TestFeatureFlagService_ClampRollout(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{-5, 0},
		{0, 0},
		{50, 50},
		{100, 100},
		{150, 100},
	}
	for _, c := range cases {
		if got := clampRollout(c.in); got != c.want {
			t.Errorf("clampRollout(%d)=%d want %d", c.in, got, c.want)
		}
	}
}

func TestFeatureFlagService_CreateAndGetByKey(t *testing.T) {
	ctx := context.Background()
	s, _ := newFeatureFlagService(t)

	rules, _ := json.Marshal(map[string]any{"users": []int64{1, 2}})
	f, err := s.Create(ctx, &CreateFlagInput{
		Key:     "feat_a",
		Enabled: 1,
		Rollout: 150, // 超界,期望被 clamp 到 100
		Rules:   rules,
	}, 7)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.Rollout != 100 {
		t.Fatalf("rollout not clamped: %d", f.Rollout)
	}
	got, err := s.GetByKey(ctx, "feat_a")
	if err != nil {
		t.Fatalf("get by key: %v", err)
	}
	if got.ID != f.ID {
		t.Fatalf("id mismatch: got %d want %d", got.ID, f.ID)
	}
}

// helper
func mustCreateFlag(t *testing.T, r *repo.Repositories, f *model.FeatureFlag) {
	t.Helper()
	if err := r.FeatureFlag.Create(context.Background(), f); err != nil {
		t.Fatalf("create flag %s: %v", f.Key, err)
	}
}
