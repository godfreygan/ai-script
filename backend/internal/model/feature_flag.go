package model

import "time"

// FeatureFlag 灰度/特性开关 — 控制功能可见性
type FeatureFlag struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"size:64;uniqueIndex" json:"key"`
	Description string    `gorm:"size:255" json:"description"`
	Enabled     int8      `gorm:"default:0" json:"enabled"`
	Rollout     int       `gorm:"default:0" json:"rollout"` // 0-100,百分比灰度
	Rules       JSON      `gorm:"type:json" json:"rules"`   // {users:[1,2],depts:[10],projects:[5]}
	CreatedBy   int64     `json:"created_by"`
	UpdatedBy   int64     `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (FeatureFlag) TableName() string { return "feature_flags" }
