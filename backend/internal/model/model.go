package model

import (
	"time"

	"gorm.io/gorm"
)

// Base 通用字段
type Base struct {
	ID        int64          `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type AuditFields struct {
	CreatedBy int64 `json:"created_by"`
	UpdatedBy int64 `json:"updated_by"`
}

// 用户
type User struct {
	Base
	Username     string     `gorm:"uniqueIndex;size:64" json:"username"`
	PasswordHash string     `gorm:"size:128" json:"-"`
	Nickname     string     `gorm:"size:64" json:"nickname"`
	Email        string     `gorm:"size:128" json:"email"`
	Phone        string     `gorm:"size:20"  json:"phone"`
	AvatarURL    string     `gorm:"size:512" json:"avatar_url"`
	DeptID       int64      `gorm:"index" json:"dept_id"`
	Status       int8       `gorm:"default:1" json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	LastLoginIP  string     `gorm:"size:64" json:"last_login_ip"`
}

func (User) TableName() string { return "users" }

// 部门
type Department struct {
	Base
	Name     string `gorm:"size:64" json:"name"`
	ParentID int64  `gorm:"index" json:"parent_id"`
	Path     string `gorm:"size:255;index" json:"path"`
	Sort     int    `json:"sort"`
	Status   int8   `gorm:"default:1" json:"status"`
}

func (Department) TableName() string { return "departments" }

// 角色
type Role struct {
	Base
	Code        string `gorm:"uniqueIndex;size:64" json:"code"`
	Name        string `gorm:"size:64" json:"name"`
	Description string `gorm:"size:255" json:"description"`
	DataScope   string `gorm:"size:16;default:'self'" json:"data_scope"`
	IsSystem    int8   `json:"is_system"`
	Status      int8   `gorm:"default:1" json:"status"`
}

func (Role) TableName() string { return "roles" }

// 项目
type Project struct {
	Base
	AuditFields
	Code              string `gorm:"uniqueIndex;size:64" json:"code"`
	Name              string `gorm:"size:128" json:"name"`
	Description       string `gorm:"type:text" json:"description"`
	OwnerID           int64  `gorm:"index" json:"owner_id"`
	DeptID            int64  `gorm:"index" json:"dept_id"`
	Status            int8   `gorm:"default:1;index" json:"status"`
	DefaultPipelineID int64  `json:"default_pipeline_id"`
	CoverURL          string `gorm:"size:512" json:"cover_url"`
	Tags              JSON   `gorm:"type:json" json:"tags"`
}

func (Project) TableName() string { return "projects" }

// 模型注册
type Model struct {
	Base
	Code             string     `gorm:"uniqueIndex;size:64" json:"code"`
	Name             string     `gorm:"size:128" json:"name"`
	Type             string     `gorm:"size:16;index" json:"type"` // text/image/video/audio
	Provider         string     `gorm:"size:32;index" json:"provider"`
	Endpoint         string     `gorm:"size:512" json:"endpoint"`
	APIKeyEncrypted  []byte     `gorm:"column:api_key_encrypted" json:"-"`
	DefaultParams    JSON       `gorm:"type:json" json:"default_params"`
	CapabilityTags   JSON       `gorm:"type:json" json:"capability_tags"`
	Enabled          int8       `gorm:"default:1;index" json:"enabled"`
	Priority         int        `json:"priority"`
	MaxQPS           int        `json:"max_qps"`
	HealthCheckURL   string     `gorm:"size:512" json:"health_check_url"`
	LastHealthAt     *time.Time `json:"last_health_at"`
	LastHealthStatus int8       `json:"last_health_status"`
}

func (Model) TableName() string { return "models" }

// 流水线
type Pipeline struct {
	Base
	ProjectID   int64  `gorm:"index" json:"project_id"`
	Name        string `gorm:"size:128" json:"name"`
	Description string `gorm:"size:512" json:"description"`
	DAG         JSON   `gorm:"type:json" json:"dag"`
	IsTemplate  int8   `json:"is_template"`
	Enabled     int8   `gorm:"default:1" json:"enabled"`
	CreatedBy   int64  `json:"created_by"`
}

func (Pipeline) TableName() string { return "pipelines" }

type PipelineRun struct {
	Base
	PipelineID  int64      `gorm:"index" json:"pipeline_id"`
	ProjectID   int64      `gorm:"index" json:"project_id"`
	TriggeredBy int64      `json:"triggered_by"`
	TriggerType string     `gorm:"size:16" json:"trigger_type"`
	Input       JSON       `gorm:"type:json" json:"input"`
	Output      JSON       `gorm:"type:json" json:"output"`
	Status      string     `gorm:"size:16;index" json:"status"`
	StartedAt   *time.Time `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
	ErrorMsg    string     `gorm:"size:1024" json:"error_msg"`
}

func (PipelineRun) TableName() string { return "pipeline_runs" }

type StepRun struct {
	Base
	RunID     int64      `gorm:"index" json:"run_id"`
	NodeID    string     `gorm:"size:64" json:"node_id"`
	NodeType  string     `gorm:"size:64" json:"node_type"`
	ModelID   int64      `gorm:"index" json:"model_id"`
	Input     JSON       `gorm:"type:json" json:"input"`
	Output    JSON       `gorm:"type:json" json:"output"`
	Status    string     `gorm:"size:16;index" json:"status"`
	Attempt   int        `json:"attempt"`
	StartedAt *time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
	ErrorMsg  string     `gorm:"size:1024" json:"error_msg"`
}

func (StepRun) TableName() string { return "step_runs" }

// 其他实体(Script/Episode/Prompt/Storyboard/Style/Image/ShortVideo/FullVideo/Review*/Publish/Quota/Invocation/Daily)
// 见 docs/database-design.md;同样模式,这里省略以控制单文件大小,实际开发时按需补全。
