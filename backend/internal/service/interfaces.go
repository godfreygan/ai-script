package service

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/adapter"
	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/storage"
	"github.com/godfreygan/ai-script/backend/pkg/ws"
	"github.com/hibiken/asynq"
)

type AuditService interface {
	Log(ctx context.Context, p *LogAuditParams)
	List(ctx context.Context, q *repo.ListAuditQuery) ([]model.AuditLog, int64, error)
}

type AuthService interface {
	Login(ctx context.Context, username, password, clientIP string) (*LoginResult, error)
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	ChangePassword(ctx context.Context, uid int64, oldPw, newPw string) error
	Logout(ctx context.Context, token string) error
}

type BillingService interface {
	ListQuotas(ctx context.Context, scopeType string, scopeID int64) ([]model.BillingQuota, error)
	GetQuota(ctx context.Context, id int64) (*model.BillingQuota, error)
	CreateQuota(ctx context.Context, in *CreateQuotaInput) (*model.BillingQuota, error)
	UpdateQuota(ctx context.Context, id int64, in *UpdateQuotaInput) (*model.BillingQuota, error)
	DeleteQuota(ctx context.Context, id int64) error
	CheckQuota(ctx context.Context, userID, deptID, modelID int64, metric string, delta float64) error
	Rollup(ctx context.Context, p *RollupParams) error
	ListDaily(ctx context.Context, from, to time.Time, userID, deptID, modelID int64) ([]model.BillingDaily, error)
}

type DeptService interface {
	List(ctx context.Context) ([]model.Department, error)
	Get(ctx context.Context, id int64) (*model.Department, error)
	Create(ctx context.Context, in *CreateDeptInput) (*model.Department, error)
	Update(ctx context.Context, id int64, name string, sort int, status int8) (*model.Department, error)
	Delete(ctx context.Context, id int64) error
}

type FeatureFlagService interface {
	List(ctx context.Context) ([]model.FeatureFlag, error)
	Get(ctx context.Context, id int64) (*model.FeatureFlag, error)
	GetByKey(ctx context.Context, key string) (*model.FeatureFlag, error)
	Create(ctx context.Context, in *CreateFlagInput, uid int64) (*model.FeatureFlag, error)
	Update(ctx context.Context, id int64, in *UpdateFlagInput, uid int64) (*model.FeatureFlag, error)
	Delete(ctx context.Context, id int64) error
	Evaluate(ctx context.Context, key string, fc *FlagContext) (bool, error)
}

type FullVideoService interface {
	SetDeps(hub *ws.Hub, store storage.Storage, modelSvc ModelService, invSvc InvocationService)
	List(ctx context.Context, q *repo.ListFullVideosQuery) ([]model.FullVideo, int64, error)
	Get(ctx context.Context, id int64) (*model.FullVideo, error)
	Create(ctx context.Context, in *CreateFullVideoInput, uid int64) (*model.FullVideo, error)
	Update(ctx context.Context, id int64, in *UpdateFullVideoInput, uid int64) (*model.FullVideo, error)
	Delete(ctx context.Context, id int64) error
	HandleComposeTask() asynq.HandlerFunc
	Render(ctx context.Context, fullID, uid int64) (string, error)
}

type ImageService interface {
	List(ctx context.Context, q *repo.ListImagesQuery) ([]model.Image, int64, error)
	Get(ctx context.Context, id int64) (*model.Image, error)
	Delete(ctx context.Context, id int64) error
	Generate(ctx context.Context, in *ImageGenInput, uid, deptID int64) (string, error)
	HandleGenerateTask(modelSvc ModelService, invSvc InvocationService) asynq.HandlerFunc
}

type InvocationService interface {
	Log(ctx context.Context, p *LogParams)
	List(ctx context.Context, q *repo.ListInvocationsQuery) ([]model.ModelInvocation, int64, error)
	Stats(ctx context.Context, q *repo.ListInvocationsQuery) (*repo.InvocationStats, error)
}

type ModelService interface {
	List(ctx context.Context, q *repo.ListModelsQuery) ([]model.Model, int64, error)
	Get(ctx context.Context, id int64) (*model.Model, error)
	Create(ctx context.Context, in *CreateModelInput) (*model.Model, error)
	Update(ctx context.Context, id int64, in *UpdateModelInput) (*model.Model, error)
	Delete(ctx context.Context, id int64) error
	Healthcheck(ctx context.Context, id int64) (bool, error)
	GetAdapter(ctx context.Context, id int64) (adapter.Adapter, *model.Model, error)
	LoadAllAdapters(ctx context.Context) error
}

type PipelineService interface {
	SetDeps(hub *ws.Hub, registry pipelineRegistry)
	List(ctx context.Context, q *repo.ListPipelinesQuery) ([]model.Pipeline, int64, error)
	Get(ctx context.Context, id int64) (*model.Pipeline, error)
	Create(ctx context.Context, in *CreatePipelineInput, uid int64) (*model.Pipeline, error)
	Update(ctx context.Context, id int64, in *UpdatePipelineInput) (*model.Pipeline, error)
	Delete(ctx context.Context, id int64) error
	ListRuns(ctx context.Context, pipelineID int64, page, size int) ([]model.PipelineRun, int64, error)
	GetRun(ctx context.Context, runID int64) (*model.PipelineRun, error)
	ListSteps(ctx context.Context, runID int64) ([]model.StepRun, error)
	HandleRunTask() asynq.HandlerFunc
	Run(ctx context.Context, pipelineID int64, input map[string]any, overrides map[string]any) (int64, error)
}

type ProjectService interface {
	List(ctx context.Context, q *repo.ListProjectsQuery) ([]model.Project, int64, error)
	Create(ctx context.Context, in *CreateProjectInput, uid, deptID int64) (*model.Project, error)
	Get(ctx context.Context, id int64) (*model.Project, error)
	Update(ctx context.Context, id int64, in *UpdateProjectInput, uid int64) (*model.Project, error)
	Delete(ctx context.Context, id int64) error
	ListMembers(ctx context.Context, projectID int64) ([]model.ProjectMember, error)
	AddMember(ctx context.Context, projectID, userID int64, roleInProject string) error
	RemoveMember(ctx context.Context, projectID, userID int64) error
}

type PromptService interface {
	Generate(ctx context.Context, episodeID int64, in *GeneratePromptInput, uid int64) (string, error)
	ListByEpisode(ctx context.Context, episodeID int64) ([]model.EpisodePrompt, error)
	GetCurrent(ctx context.Context, episodeID int64) (*model.EpisodePrompt, error)
	SetCurrent(ctx context.Context, episodeID, id int64) error
	HandleGenerateTask(modelSvc ModelService) asynq.HandlerFunc
}

type PublishService interface {
	Publish(ctx context.Context, in *PublishInput, uid int64) (*model.Publish, error)
	Unpublish(ctx context.Context, videoID int64) error
	Get(ctx context.Context, publishID int64) (*model.Publish, error)
	List(ctx context.Context, status string, page, size int) ([]model.Publish, int64, error)
	IncPlay(ctx context.Context, videoID int64) error
	IncDownload(ctx context.Context, videoID int64) error
	UpdateWatermark(ctx context.Context, videoID int64, raw json.RawMessage) (*model.Publish, error)
}

type ReviewService interface {
	ListFlows(ctx context.Context) ([]model.ReviewFlow, error)
	GetFlow(ctx context.Context, id int64) (*model.ReviewFlow, error)
	ListNodes(ctx context.Context, flowID int64) ([]model.ReviewNode, error)
	Submit(ctx context.Context, in *SubmitInput, uid int64) (*model.ReviewRecord, error)
	ListRecords(ctx context.Context, status string, page, size int) ([]model.ReviewRecord, int64, error)
	GetRecord(ctx context.Context, id int64) (*model.ReviewRecord, error)
	ListActions(ctx context.Context, recordID int64) ([]model.ReviewNodeRecord, error)
	Act(ctx context.Context, recordID int64, in *ActInput, uid int64) (*model.ReviewRecord, error)
	Cancel(ctx context.Context, recordID int64, uid int64) error
}

type RoleService interface {
	List(ctx context.Context) ([]model.Role, error)
	Get(ctx context.Context, id int64) (*RoleWithPermissions, error)
	ListPermissions(ctx context.Context) ([]model.Permission, error)
	Create(ctx context.Context, in *CreateRoleInput) (*model.Role, error)
	Update(ctx context.Context, id int64, in *UpdateRoleInput) (*model.Role, error)
	Delete(ctx context.Context, id int64) error
	SyncCasbin(ctx context.Context) error
}

type ScriptService interface {
	List(ctx context.Context, q *repo.ListScriptsQuery) ([]model.Script, int64, error)
	Get(ctx context.Context, id int64) (*model.Script, error)
	Create(ctx context.Context, in *CreateScriptInput, uid int64) (*model.Script, error)
	Delete(ctx context.Context, id int64) error
	ListEpisodes(ctx context.Context, scriptID int64) ([]model.Episode, error)
	Split(ctx context.Context, scriptID int64, in *SplitScriptInput, uid int64) (string, error)
	HandleSplitTask(modelSvc ModelService) asynq.HandlerFunc
}

type ShortVideoService interface {
	List(ctx context.Context, q *repo.ListShortVideosQuery) ([]model.ShortVideo, int64, error)
	Get(ctx context.Context, id int64) (*model.ShortVideo, error)
	Delete(ctx context.Context, id int64) error
	Generate(ctx context.Context, in *ShortVideoGenInput, uid, deptID int64) (string, error)
	HandleGenerateTask(modelSvc ModelService, invSvc InvocationService) asynq.HandlerFunc
}

type StoryboardService interface {
	ListByEpisode(ctx context.Context, episodeID int64) ([]model.Storyboard, error)
	Get(ctx context.Context, id int64) (*model.Storyboard, error)
	Update(ctx context.Context, sb *model.Storyboard) error
	Delete(ctx context.Context, id int64) error
	BulkSave(ctx context.Context, episodeID int64, list []model.Storyboard) error
	ApplyStyle(ctx context.Context, storyboardID, styleID, userID int64) error
	Generate(ctx context.Context, episodeID, modelID int64, params map[string]any, uid int64) (string, error)
	HandleGenerateTask(modelSvc ModelService) asynq.HandlerFunc
}

type StyleService interface {
	List(ctx context.Context, projectID int64) ([]model.Style, error)
	Get(ctx context.Context, id int64) (*model.Style, error)
	Create(ctx context.Context, in *CreateStyleInput, uid int64) (*model.Style, error)
	Update(ctx context.Context, id int64, in *UpdateStyleInput) (*model.Style, error)
	Delete(ctx context.Context, id int64) error
}

type UploadService interface {
	Save(ctx context.Context, namespace, originalName, contentType string, r io.Reader, size int64) (*UploadResult, error)
}

type UserService interface {
	Me(ctx context.Context, uid int64) (*model.User, error)
	List(ctx context.Context, q *repo.ListUsersQuery) ([]model.User, int64, error)
	Get(ctx context.Context, id int64) (*UserWithRoles, error)
	Create(ctx context.Context, in *CreateUserInput) (*model.User, error)
	Update(ctx context.Context, id int64, in *UpdateUserInput) (*model.User, error)
	Delete(ctx context.Context, id int64) error
	ResetPassword(ctx context.Context, id int64, newPw string) error
}
