package repo

import (
	"context"
	"errors"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
)

// ErrSeedFailed 当 AutoMigrate 成功但 Seed 失败时,Migrate 仍然返回 nil(不阻止启动);
// 这个变量保留给后续观测使用,目前直接被 Migrate 内部 swallow 后写日志(由调用方写)。
// 这里保留 sentinel 仅作类型断言用,不会被实际返回。
var ErrSeedFailed = errors.New("seed failed")

// Migrate 跑 GORM AutoMigrate 把所有业务表补齐,然后写入默认 seed 数据。
// 注意:AutoMigrate 只增不删,生产环境慎用——但 MVP 阶段可启动时直接调。
// Seed 必须幂等,Seed 失败不会阻止启动(被 swallow,仅打印到 stderr/log)。
func (r *Repositories) Migrate(ctx context.Context) error {
	if err := r.DB.WithContext(ctx).AutoMigrate(
		// 基础域
		&model.User{}, &model.Department{}, &model.Role{}, &model.Project{},
		&model.UserAPIToken{}, &model.Permission{}, &model.RolePermission{}, &model.UserRole{},
		&model.ProjectMember{}, &model.Model{},

		// 内容生产域
		&model.Script{}, &model.ScriptVersion{}, &model.Episode{}, &model.EpisodePrompt{},
		&model.Storyboard{}, &model.Style{}, &model.StoryboardStyle{},
		&model.Image{}, &model.ShortVideo{}, &model.FullVideo{},

		// 流水线
		&model.Pipeline{}, &model.PipelineRun{}, &model.StepRun{},

		// 审核 / 发布
		&model.ReviewFlow{}, &model.ReviewNode{}, &model.ReviewRecord{}, &model.ReviewNodeRecord{},
		&model.Publish{},

		// 计费 / 调用
		&model.ModelPricing{}, &model.ModelInvocation{},
		&model.BillingQuota{}, &model.BillingDaily{},

		// 系统
		&model.AuditLog{}, &model.SysDict{}, &model.FeatureFlag{},
	); err != nil {
		return err
	}
	// Seed 必须幂等,即便失败也不能阻止启动 —— 通过 GORM 的 logger 已经把详细错误打到日志,
	// 这里只在 stdout 留一行警告,供 server.go 的 log.Warn("auto-migrate failed") 链路兜底。
	if err := r.Seed(ctx); err != nil {
		// 把 seed 错误用 GORM logger 输出(GORM 默认 logger 会写到 stderr/zap),
		// 这里避免引入 zap 依赖反向污染 repo 包。
		r.DB.Logger.Warn(ctx, "seed default data failed: %v", err)
	}
	return nil
}
