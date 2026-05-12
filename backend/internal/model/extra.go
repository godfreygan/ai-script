package model

import "time"

// =============== 用户域扩展 ===============

type UserAPIToken struct {
	Base
	UserID     int64      `gorm:"index" json:"user_id"`
	Name       string     `gorm:"size:64" json:"name"`
	TokenHash  string     `gorm:"size:128;uniqueIndex" json:"-"`
	Scopes     JSON       `gorm:"type:json" json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	Status     int8       `gorm:"default:1" json:"status"`
}

func (UserAPIToken) TableName() string { return "user_api_tokens" }

// =============== 鉴权域 ===============

type Permission struct {
	ID          int64  `gorm:"primaryKey" json:"id"`
	Code        string `gorm:"size:128;uniqueIndex" json:"code"`
	Name        string `gorm:"size:128" json:"name"`
	Resource    string `gorm:"size:64" json:"resource"`
	Action      string `gorm:"size:32" json:"action"`
	Description string `gorm:"size:255" json:"description"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Permission) TableName() string { return "permissions" }

type RolePermission struct {
	RoleID       int64 `gorm:"primaryKey" json:"role_id"`
	PermissionID int64 `gorm:"primaryKey" json:"permission_id"`
}

func (RolePermission) TableName() string { return "role_permissions" }

type UserRole struct {
	UserID int64 `gorm:"primaryKey" json:"user_id"`
	RoleID int64 `gorm:"primaryKey" json:"role_id"`
}

func (UserRole) TableName() string { return "user_roles" }

// =============== 项目域扩展 ===============

type ProjectMember struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	ProjectID     int64     `gorm:"index" json:"project_id"`
	UserID        int64     `gorm:"index" json:"user_id"`
	RoleInProject string    `gorm:"size:32;default:editor" json:"role_in_project"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ProjectMember) TableName() string { return "project_members" }

// =============== 剧本域 ===============

type Script struct {
	Base
	AuditFields
	ProjectID      int64  `gorm:"index" json:"project_id"`
	Name           string `gorm:"size:128" json:"name"`
	SourceURL      string `gorm:"size:512" json:"source_url"`
	RawText        string `gorm:"type:mediumtext" json:"raw_text"`
	CurrentVersion int    `gorm:"default:1" json:"current_version"`
	Status         int8   `gorm:"default:1" json:"status"` // 1=uploaded 2=parsed 3=episode_split
	Meta           JSON   `gorm:"type:json" json:"meta"`
}

func (Script) TableName() string { return "scripts" }

type ScriptVersion struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	ScriptID  int64     `gorm:"index" json:"script_id"`
	VersionNo int       `json:"version_no"`
	Content   string    `gorm:"type:mediumtext" json:"content"`
	Diff      JSON      `gorm:"type:json" json:"diff"`
	CommitMsg string    `gorm:"size:255" json:"commit_msg"`
	CreatedBy int64     `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (ScriptVersion) TableName() string { return "script_versions" }

type Episode struct {
	Base
	ScriptID   int64  `gorm:"index" json:"script_id"`
	EpNo       int    `json:"ep_no"`
	Title      string `gorm:"size:255" json:"title"`
	Summary    string `gorm:"type:text" json:"summary"`
	RawSegment string `gorm:"type:mediumtext" json:"raw_segment"`
	CharBegin  int    `json:"char_begin"`
	CharEnd    int    `json:"char_end"`
	Status     int8   `gorm:"default:1" json:"status"`
}

func (Episode) TableName() string { return "episodes" }

// =============== 提示词域 ===============

type EpisodePrompt struct {
	ID               int64     `gorm:"primaryKey" json:"id"`
	EpisodeID        int64     `gorm:"index" json:"episode_id"`
	Version          int       `gorm:"default:1" json:"version"`
	IsCurrent        int8      `json:"is_current"`
	Content          JSON      `gorm:"type:json" json:"content"`
	ModelID          int64     `json:"model_id"`
	GenerationParams JSON      `gorm:"type:json" json:"generation_params"`
	Status           int8      `gorm:"default:1" json:"status"`
	GeneratedBy      int64     `json:"generated_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (EpisodePrompt) TableName() string { return "episode_prompts" }

// =============== 分镜 / 风格 ===============

type Storyboard struct {
	Base
	EpisodeID    int64  `gorm:"index" json:"episode_id"`
	PromptID     int64  `gorm:"index" json:"prompt_id"`
	ShotNo       int    `json:"shot_no"`
	ShotType     string `gorm:"size:32;default:medium" json:"shot_type"`
	CameraMotion string `gorm:"size:32;default:static" json:"camera_motion"`
	SceneDesc    string `gorm:"type:text" json:"scene_desc"`
	Characters   JSON   `gorm:"type:json" json:"characters"`
	Action       string `gorm:"type:text" json:"action"`
	Dialogue     string `gorm:"type:text" json:"dialogue"`
	DurationSec  int    `gorm:"default:15" json:"duration_sec"`
	Notes        string `gorm:"type:text" json:"notes"`
	Status       int8   `gorm:"default:1" json:"status"`
}

func (Storyboard) TableName() string { return "storyboards" }

type Style struct {
	Base
	ProjectID       int64  `gorm:"index" json:"project_id"`
	Name            string `gorm:"size:64" json:"name"`
	ArtStyle        string `gorm:"size:64" json:"art_style"`
	ColorTone       string `gorm:"size:64" json:"color_tone"`
	Lighting        string `gorm:"size:64" json:"lighting"`
	ReferenceImages JSON   `gorm:"type:json" json:"reference_images"`
	LoraID          string `gorm:"size:128" json:"lora_id"`
	Description     string `gorm:"type:text" json:"description"`
	Status          int8   `gorm:"default:1" json:"status"`
	CreatedBy       int64  `json:"created_by"`
}

func (Style) TableName() string { return "styles" }

type StoryboardStyle struct {
	StoryboardID int64     `gorm:"primaryKey" json:"storyboard_id"`
	StyleID      int64     `gorm:"primaryKey" json:"style_id"`
	AppliedAt    time.Time `json:"applied_at"`
	AppliedBy    int64     `json:"applied_by"`
}

func (StoryboardStyle) TableName() string { return "storyboard_styles" }

// =============== 图片 ===============

type Image struct {
	Base
	ProjectID         int64  `gorm:"index" json:"project_id"`
	StoryboardID      int64  `gorm:"index" json:"storyboard_id"`
	SrcType           string `gorm:"size:16;default:generated" json:"src_type"`
	URL               string `gorm:"size:512" json:"url"`
	ThumbURL          string `gorm:"size:512" json:"thumb_url"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	Prompt            string `gorm:"type:text" json:"prompt"`
	NegPrompt         string `gorm:"type:text;column:neg_prompt" json:"neg_prompt"`
	ModelID           int64  `json:"model_id"`
	Params            JSON   `gorm:"type:json" json:"params"`
	Status            int8   `gorm:"default:1" json:"status"`
	GeneratedInRunID  int64  `gorm:"column:generated_in_run_id" json:"generated_in_run_id"`
	CreatedBy         int64  `json:"created_by"`
}

func (Image) TableName() string { return "images" }

// =============== 短视频 ===============

type ShortVideo struct {
	Base
	ProjectID        int64  `gorm:"index" json:"project_id"`
	StoryboardID     int64  `gorm:"index" json:"storyboard_id"`
	SrcType          string `gorm:"size:16;default:generated" json:"src_type"`
	Prompt           string `gorm:"type:text" json:"prompt"`
	SourceImageIDs   JSON   `gorm:"type:json;column:source_image_ids" json:"source_image_ids"`
	VideoURL         string `gorm:"size:512;column:video_url" json:"video_url"`
	ThumbURL         string `gorm:"size:512" json:"thumb_url"`
	DurationMs       int    `json:"duration_ms"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	AudioURL         string `gorm:"size:512" json:"audio_url"`
	SubtitleURL      string `gorm:"size:512" json:"subtitle_url"`
	ModelID          int64  `json:"model_id"`
	Params           JSON   `gorm:"type:json" json:"params"`
	Status           string `gorm:"size:16;default:queued" json:"status"`
	ErrorMsg         string `gorm:"size:512" json:"error_msg"`
	RetryCount       int    `json:"retry_count"`
	GeneratedInRunID int64  `gorm:"column:generated_in_run_id" json:"generated_in_run_id"`
	CreatedBy        int64  `json:"created_by"`
}

func (ShortVideo) TableName() string { return "short_videos" }

// =============== 完整视频 ===============

type FullVideo struct {
	Base
	AuditFields
	ProjectID      int64  `gorm:"index" json:"project_id"`
	Name           string `gorm:"size:128" json:"name"`
	Version        int    `gorm:"default:1" json:"version"`
	Timeline       JSON   `gorm:"type:json" json:"timeline"`
	OutputURL      string `gorm:"size:512;column:output_url" json:"output_url"`
	ThumbURL       string `gorm:"size:512" json:"thumb_url"`
	CoverURL       string `gorm:"size:512" json:"cover_url"`
	DurationMs     int    `json:"duration_ms"`
	Status         string `gorm:"size:16;default:draft" json:"status"`
	RenderProgress int    `json:"render_progress"`
	ErrorMsg       string `gorm:"size:512" json:"error_msg"`
}

func (FullVideo) TableName() string { return "full_videos" }

// =============== 审核 / 发布 ===============

type ReviewFlow struct {
	Base
	Name        string `gorm:"size:64" json:"name"`
	Description string `gorm:"size:255" json:"description"`
	TargetType  string `gorm:"size:32;default:full_video" json:"target_type"`
	Enabled     int8   `gorm:"default:1" json:"enabled"`
	IsDefault   int8   `gorm:"column:is_default" json:"is_default"`
}

func (ReviewFlow) TableName() string { return "review_flows" }

type ReviewNode struct {
	ID                int64  `gorm:"primaryKey" json:"id"`
	FlowID            int64  `gorm:"index" json:"flow_id"`
	StepNo            int    `json:"step_no"`
	Name              string `gorm:"size:64" json:"name"`
	ApproverType      string `gorm:"size:16" json:"approver_type"`
	ApproverValue     string `gorm:"size:64" json:"approver_value"`
	AllowTimeoutPass  int8   `gorm:"column:allow_timeout_pass" json:"allow_timeout_pass"`
	TimeoutHours      int    `gorm:"column:timeout_hours" json:"timeout_hours"`
}

func (ReviewNode) TableName() string { return "review_nodes" }

type ReviewRecord struct {
	ID          int64      `gorm:"primaryKey" json:"id"`
	TargetType  string     `gorm:"size:32;default:full_video" json:"target_type"`
	TargetID    int64      `json:"target_id"`
	FlowID      int64      `json:"flow_id"`
	CurrentStep int        `gorm:"default:1" json:"current_step"`
	Status      string     `gorm:"size:16;default:pending" json:"status"`
	SubmittedBy int64      `json:"submitted_by"`
	FinishedAt  *time.Time `json:"finished_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (ReviewRecord) TableName() string { return "review_records" }

type ReviewNodeRecord struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	ReviewRecordID int64     `gorm:"index;column:review_record_id" json:"review_record_id"`
	StepNo         int       `json:"step_no"`
	ApproverID     int64     `json:"approver_id"`
	Action         string    `gorm:"size:32" json:"action"`
	Comment        string    `gorm:"size:1024" json:"comment"`
	ActedAt        time.Time `json:"acted_at"`
}

func (ReviewNodeRecord) TableName() string { return "review_node_records" }

type Publish struct {
	ID              int64     `gorm:"primaryKey" json:"id"`
	FullVideoID     int64     `gorm:"uniqueIndex;column:full_video_id" json:"full_video_id"`
	PublishedBy     int64     `json:"published_by"`
	PublishedAt     time.Time `json:"published_at"`
	Status          string    `gorm:"size:8;default:on" json:"status"`
	WatermarkConfig JSON      `gorm:"type:json" json:"watermark_config"`
	DownloadCount   int       `json:"download_count"`
	PlayCount       int       `json:"play_count"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (Publish) TableName() string { return "publishes" }

// =============== 模型扩展 ===============

type ModelPricing struct {
	ID            int64      `gorm:"primaryKey" json:"id"`
	ModelID       int64      `gorm:"index" json:"model_id"`
	Metric        string     `gorm:"size:32" json:"metric"`
	PricePerUnit  float64    `gorm:"type:decimal(20,8)" json:"price_per_unit"`
	Currency      string     `gorm:"size:8;default:CNY" json:"currency"`
	EffectiveFrom time.Time  `json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (ModelPricing) TableName() string { return "model_pricing" }

type ModelInvocation struct {
	ID           int64      `gorm:"primaryKey" json:"id"`
	ModelID      int64      `gorm:"index" json:"model_id"`
	UserID       int64      `gorm:"index" json:"user_id"`
	DeptID       int64      `gorm:"index" json:"dept_id"`
	ProjectID    int64      `gorm:"index" json:"project_id"`
	BizType      string     `gorm:"size:16" json:"biz_type"`
	BizRef       string     `gorm:"size:64" json:"biz_ref"`
	InputTokens  int        `json:"input_tokens"`
	OutputTokens int        `json:"output_tokens"`
	Units        int        `json:"units"`
	DurationMs   int        `json:"duration_ms"`
	Cost         float64    `gorm:"type:decimal(20,8)" json:"cost"`
	Status       string     `gorm:"size:16;default:succeeded" json:"status"`
	ErrorCode    string     `gorm:"size:32" json:"error_code"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at"`
}

func (ModelInvocation) TableName() string { return "model_invocations" }

// =============== 计费 ===============

type BillingQuota struct {
	ID         int64      `gorm:"primaryKey" json:"id"`
	ScopeType  string     `gorm:"size:8" json:"scope_type"`
	ScopeID    int64      `json:"scope_id"`
	ModelID    int64      `json:"model_id"`
	Period     string     `gorm:"size:16;default:monthly" json:"period"`
	Metric     string     `gorm:"size:32" json:"metric"`
	QuotaValue float64    `gorm:"type:decimal(20,4)" json:"quota_value"`
	UsedValue  float64    `gorm:"type:decimal(20,4)" json:"used_value"`
	ResetAt    *time.Time `json:"reset_at"`
	Enabled    int8       `gorm:"default:1" json:"enabled"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (BillingQuota) TableName() string { return "billing_quotas" }

type BillingDaily struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	StatDate     time.Time `gorm:"type:date;uniqueIndex:uniq_daily_dim,priority:1" json:"stat_date"`
	ModelID      int64     `gorm:"uniqueIndex:uniq_daily_dim,priority:2" json:"model_id"`
	DeptID       int64     `gorm:"uniqueIndex:uniq_daily_dim,priority:3" json:"dept_id"`
	UserID       int64     `gorm:"uniqueIndex:uniq_daily_dim,priority:4" json:"user_id"`
	Calls        int       `json:"calls"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	Units        int64     `json:"units"`
	Cost         float64   `gorm:"type:decimal(20,8)" json:"cost"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (BillingDaily) TableName() string { return "billing_daily" }

// =============== 系统域 ===============

type AuditLog struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	UserID       int64     `gorm:"index" json:"user_id"`
	Action       string    `gorm:"size:64" json:"action"`
	ResourceType string    `gorm:"size:64" json:"resource_type"`
	ResourceID   string    `gorm:"size:64" json:"resource_id"`
	Before       JSON      `gorm:"type:json" json:"before"`
	After        JSON      `gorm:"type:json" json:"after"`
	IP           string    `gorm:"size:64" json:"ip"`
	UA           string    `gorm:"size:255" json:"ua"`
	RequestID    string    `gorm:"size:64" json:"request_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }

type SysDict struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"size:64;uniqueIndex:uk_type_code" json:"type"`
	Code      string    `gorm:"size:64;uniqueIndex:uk_type_code" json:"code"`
	Name      string    `gorm:"size:128" json:"name"`
	Value     string    `gorm:"size:255" json:"value"`
	Sort      int       `json:"sort"`
	Status    int8      `gorm:"default:1" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SysDict) TableName() string { return "sys_dicts" }
