package service

import (
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/adapter"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/crypto"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/jwt"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/queue"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/storage"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/ws"
	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"
)

// Services 业务服务集合
type Services struct {
	Auth        AuthService
	User        UserService
	Dept        DeptService
	Role        RoleService
	Project     ProjectService
	Model       ModelService
	Script      ScriptService
	Prompt      PromptService
	Story       StoryboardService
	Style       StyleService
	Image       ImageService
	Short       ShortVideoService
	Full        FullVideoService
	Pipeline    PipelineService
	Upload      UploadService
	Invoke      InvocationService
	Review      ReviewService
	Publish     PublishService
	Billing     BillingService
	Audit       AuditService
	FeatureFlag FeatureFlagService
}

// NewServices 构造业务服务集合;hub 可为 nil(纯 worker 进程或测试),store 也可为 nil(无上传场景)
func NewServices(
	r *repo.Repositories,
	jwtMgr *jwt.Manager,
	enforcer *casbin.Enforcer,
	cipher *crypto.Cipher,
	registry *adapter.Registry,
	taskClient queue.TaskClient,
	hub *ws.Hub,
	store storage.Storage,
	log *zap.Logger,
) *Services {
	if taskClient == nil {
		taskClient = &queue.NoopTaskClient{}
	}
	// 注入 Redis 黑名单检查器（若 rdb 可用）
	if r.RDB != nil {
		jwtMgr.SetRevocationChecker(&jwt.RedisRevocationChecker{RDB: r.RDB})
	}

	s := &Services{
		Auth:        &authService{user: r.User, jwt: jwtMgr, log: log, rdb: r.RDB},
		User:        &userService{user: r.User, role: r.Role, log: log},
		Dept:        &deptService{dept: r.Dept, log: log},
		Role:        &roleService{role: r.Role, db: r.DB, enforcer: enforcer, log: log},
		Project:     &projectService{project: r.Project, log: log},
		Model:       &modelService{r: r, cipher: cipher, registry: registry, log: log},
		Script:      &scriptService{r: r, tc: taskClient, hub: hub, log: log},
		Prompt:      &promptService{r: r, tc: taskClient, hub: hub, log: log},
		Story:       &storyboardService{r: r, tc: taskClient, hub: hub, log: log},
		Style:       &styleService{r: r, log: log},
		Image:       &imageService{r: r, tc: taskClient, hub: hub, store: store, log: log},
		Short:       &shortVideoService{r: r, tc: taskClient, hub: hub, store: store, log: log},
		Full:        &fullVideoService{r: r, tc: taskClient, hub: hub, store: store, modelSvc: nil, invSvc: nil, log: log},
		Pipeline:    &pipelineService{r: r, db: r.DB, tc: taskClient, hub: hub, registry: nil, log: log},
		Upload:      NewUploadService(store, log),
		Invoke:      &invocationService{r: r, log: log},
		Review:      &reviewService{r: r, log: log},
		Publish:     &publishService{r: r, log: log},
		Billing:     &billingService{r: r, log: log},
		Audit:       &auditService{r: r, log: log},
		FeatureFlag: &featureFlagService{r: r, log: log},
	}
	// 把 hub/store/modelSvc/invSvc 注入 Full,这样 server 端 publish 与 worker 端 compose 都能工作
	s.Full.SetDeps(hub, store, s.Model, s.Invoke)
	// Pipeline 的 registry 留待 worker 端用 *pipeline.Runner 注入;server 端 hub 现在就注入,确保 ws 主题 pipeline:* 能被订阅
	s.Pipeline.SetDeps(hub, nil)
	return s
}
