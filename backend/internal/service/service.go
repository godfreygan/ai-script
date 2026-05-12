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
	Auth        *AuthService
	User        *UserService
	Dept        *DeptService
	Role        *RoleService
	Project     *ProjectService
	Model       *ModelService
	Script      *ScriptService
	Prompt      *PromptService
	Story       *StoryboardService
	Style       *StyleService
	Image       *ImageService
	Short       *ShortVideoService
	Full        *FullVideoService
	Pipeline    *PipelineService
	Upload      *UploadService
	Invoke      *InvocationService
	Review      *ReviewService
	Publish     *PublishService
	Billing     *BillingService
	Audit       *AuditService
	FeatureFlag *FeatureFlagService
}

// NewServices 构造业务服务集合;hub 可为 nil(纯 worker 进程或测试),store 也可为 nil(无上传场景)
func NewServices(
	r *repo.Repositories,
	jwtMgr *jwt.Manager,
	enforcer *casbin.Enforcer,
	cipher *crypto.Cipher,
	registry *adapter.Registry,
	taskClient *queue.Client,
	hub *ws.Hub,
	store storage.Storage,
	log *zap.Logger,
) *Services {
	s := &Services{
		Auth:        &AuthService{user: r.User, jwt: jwtMgr, log: log},
		User:        &UserService{user: r.User, role: r.Role, log: log},
		Dept:        &DeptService{dept: r.Dept, log: log},
		Role:        &RoleService{role: r.Role, enforcer: enforcer, log: log},
		Project:     &ProjectService{project: r.Project, log: log},
		Model:       &ModelService{r: r, cipher: cipher, registry: registry, log: log},
		Script:      &ScriptService{r: r, tc: taskClient, hub: hub, log: log},
		Prompt:      &PromptService{r: r, tc: taskClient, hub: hub, log: log},
		Story:       &StoryboardService{r: r, tc: taskClient, hub: hub, log: log},
		Style:       &StyleService{r: r, log: log},
		Image:       &ImageService{r: r, tc: taskClient, hub: hub, store: store, log: log},
		Short:       &ShortVideoService{r: r, tc: taskClient, hub: hub, store: store, log: log},
		Full:        &FullVideoService{r: r, tc: taskClient, log: log},
		Pipeline:    &PipelineService{r: r, tc: taskClient, log: log},
		Upload:      NewUploadService(store, log),
		Invoke:      &InvocationService{r: r, log: log},
		Review:      &ReviewService{r: r, log: log},
		Publish:     &PublishService{r: r, log: log},
		Billing:     &BillingService{r: r, log: log},
		Audit:       &AuditService{r: r, log: log},
		FeatureFlag: &FeatureFlagService{r: r, log: log},
	}
	// 把 hub/store/modelSvc/invSvc 注入 Full,这样 server 端 publish 与 worker 端 compose 都能工作
	s.Full.SetDeps(hub, store, s.Model, s.Invoke)
	// Pipeline 的 registry 留待 worker 端用 *pipeline.Runner 注入;server 端 hub 现在就注入,确保 ws 主题 pipeline:* 能被订阅
	s.Pipeline.SetDeps(hub, nil)
	return s
}
