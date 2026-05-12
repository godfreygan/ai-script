package repo

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Repositories 仓储集合(每个域一个 repo)
type Repositories struct {
	DB  *gorm.DB
	RDB *redis.Client

	User       *UserRepo
	Dept       *DeptRepo
	Role       *RoleRepo
	Project    *ProjectRepo
	Script     *ScriptRepo
	Episode    *EpisodeRepo
	Prompt     *PromptRepo
	Story      *StoryboardRepo
	Style      *StyleRepo
	Image      *ImageRepo
	Short      *ShortVideoRepo
	Full       *FullVideoRepo
	Model      *ModelRepo
	Invocation *InvocationRepo
	Pipeline   *PipelineRepo
	Review      *ReviewRepo
	Publish     *PublishRepo
	Billing     *BillingRepo
	Audit       *AuditRepo
	FeatureFlag *FeatureFlagRepo
}

func NewRepositories(db *gorm.DB, rdb *redis.Client) *Repositories {
	return &Repositories{
		DB:         db,
		RDB:        rdb,
		User:       &UserRepo{db: db},
		Dept:       &DeptRepo{db: db},
		Role:       &RoleRepo{db: db},
		Project:    &ProjectRepo{db: db},
		Script:     &ScriptRepo{db: db},
		Episode:    &EpisodeRepo{db: db},
		Prompt:     &PromptRepo{db: db},
		Story:      &StoryboardRepo{db: db},
		Style:      &StyleRepo{db: db},
		Image:      &ImageRepo{db: db},
		Short:      &ShortVideoRepo{db: db},
		Full:       &FullVideoRepo{db: db},
		Model:      &ModelRepo{db: db},
		Invocation: &InvocationRepo{db: db},
		Pipeline:   &PipelineRepo{db: db},
		Review:      &ReviewRepo{db: db},
		Publish:     &PublishRepo{db: db},
		Billing:     &BillingRepo{db: db},
		Audit:       &AuditRepo{db: db},
		FeatureFlag: &FeatureFlagRepo{db: db},
	}
}
