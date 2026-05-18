package handler

import (
	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/ws"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Handlers struct {
	Auth        *AuthHandler
	User        *UserHandler
	Dept        *DeptHandler
	Role        *RoleHandler
	Project     *ProjectHandler
	Model       *ModelHandler
	Script      *ScriptHandler
	Prompt      *PromptHandler
	Story       *StoryboardHandler
	Style       *StyleHandler
	Image       *ImageHandler
	Short       *ShortVideoHandler
	Full        *FullVideoHandler
	Pipeline    *PipelineHandler
	Upload      *UploadHandler
	Invocation  *InvocationHandler
	WS          *WSHandler
	Review      *ReviewHandler
	Publish     *PublishHandler
	Billing     *BillingHandler
	Audit       *AuditHandler
	FeatureFlag *FeatureFlagHandler
}

func NewHandlers(s *service.Services, hub *ws.Hub, log *zap.Logger, rdb *redis.Client) *Handlers {
	return &Handlers{
		Auth:        &AuthHandler{auth: s.Auth, log: log},
		User:        &UserHandler{user: s.User, auth: s.Auth, log: log},
		Dept:        &DeptHandler{dept: s.Dept, log: log},
		Role:        &RoleHandler{role: s.Role, log: log},
		Project:     &ProjectHandler{project: s.Project, log: log},
		Model:       &ModelHandler{model: s.Model, log: log},
		Script:      &ScriptHandler{script: s.Script, log: log},
		Prompt:      &PromptHandler{prompt: s.Prompt, log: log},
		Story:       &StoryboardHandler{story: s.Story, log: log},
		Style:       &StyleHandler{style: s.Style, log: log},
		Image:       &ImageHandler{img: s.Image, log: log},
		Short:       &ShortVideoHandler{short: s.Short, log: log},
		Full:        &FullVideoHandler{full: s.Full, log: log},
		Pipeline:    &PipelineHandler{pipeline: s.Pipeline, log: log},
		Upload:      &UploadHandler{upload: s.Upload, log: log},
		Invocation:  &InvocationHandler{invoke: s.Invoke, log: log},
		WS:          &WSHandler{hub: hub, log: log},
		Review:      &ReviewHandler{review: s.Review, log: log},
		Publish:     &PublishHandler{publish: s.Publish, log: log},
		Billing:     &BillingHandler{billing: s.Billing, log: log},
		Audit:       &AuditHandler{audit: s.Audit, log: log},
		FeatureFlag: &FeatureFlagHandler{flag: s.FeatureFlag, log: log},
	}
}
