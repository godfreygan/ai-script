package service

import (
	"context"
	"encoding/json"
	"errors"
	"hash/crc32"
	"strconv"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type featureFlagService struct {
	r   *repo.Repositories
	log *zap.Logger
}

type CreateFlagInput struct {
	Key         string          `json:"key" binding:"required"`
	Description string          `json:"description"`
	Enabled     int8            `json:"enabled"`
	Rollout     int             `json:"rollout"`
	Rules       json.RawMessage `json:"rules"`
}

type UpdateFlagInput struct {
	Description *string         `json:"description"`
	Enabled     *int8           `json:"enabled"`
	Rollout     *int            `json:"rollout"`
	Rules       json.RawMessage `json:"rules"`
}

type FlagContext struct {
	UserID    int64
	DeptID    int64
	ProjectID int64
}

type flagRules struct {
	Users    []int64 `json:"users"`
	Depts    []int64 `json:"depts"`
	Projects []int64 `json:"projects"`
}

// clampRollout 把 rollout 钳到 [0,100]
func clampRollout(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func (s *featureFlagService) List(ctx context.Context) ([]model.FeatureFlag, error) {
	return s.r.FeatureFlag.List(ctx)
}

func (s *featureFlagService) Get(ctx context.Context, id int64) (*model.FeatureFlag, error) {
	f, err := s.r.FeatureFlag.Get(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

func (s *featureFlagService) GetByKey(ctx context.Context, key string) (*model.FeatureFlag, error) {
	f, err := s.r.FeatureFlag.GetByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errcode.ErrNotFound
	}
	return f, nil
}

func (s *featureFlagService) Create(ctx context.Context, in *CreateFlagInput, uid int64) (*model.FeatureFlag, error) {
	f := &model.FeatureFlag{
		Key:         in.Key,
		Description: in.Description,
		Enabled:     in.Enabled,
		Rollout:     clampRollout(in.Rollout),
		CreatedBy:   uid,
		UpdatedBy:   uid,
	}
	if len(in.Rules) > 0 {
		f.Rules = model.JSON(in.Rules)
	}
	if err := s.r.FeatureFlag.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *featureFlagService) Update(ctx context.Context, id int64, in *UpdateFlagInput, uid int64) (*model.FeatureFlag, error) {
	f, err := s.r.FeatureFlag.Get(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, err
	}
	if in.Description != nil {
		f.Description = *in.Description
	}
	if in.Enabled != nil {
		f.Enabled = *in.Enabled
	}
	if in.Rollout != nil {
		f.Rollout = clampRollout(*in.Rollout)
	}
	if len(in.Rules) > 0 {
		f.Rules = model.JSON(in.Rules)
	}
	f.UpdatedBy = uid
	if err := s.r.FeatureFlag.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *featureFlagService) Delete(ctx context.Context, id int64) error {
	err := s.r.FeatureFlag.Delete(ctx, id)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

// Evaluate 评估 key 对应的特性开关是否对当前 context 开启
func (s *featureFlagService) Evaluate(ctx context.Context, key string, fc *FlagContext) (bool, error) {
	f, err := s.r.FeatureFlag.GetByKey(ctx, key)
	if err != nil {
		return false, err
	}
	if f == nil || f.Enabled == 0 {
		return false, nil
	}
	// 反序列化规则
	if len(f.Rules) > 0 {
		var rules flagRules
		if err := json.Unmarshal(f.Rules, &rules); err == nil && fc != nil {
			for _, u := range rules.Users {
				if u == fc.UserID && fc.UserID != 0 {
					return true, nil
				}
			}
			for _, d := range rules.Depts {
				if d == fc.DeptID && fc.DeptID != 0 {
					return true, nil
				}
			}
			for _, p := range rules.Projects {
				if p == fc.ProjectID && fc.ProjectID != 0 {
					return true, nil
				}
			}
		}
	}
	// 规则未命中,按 Rollout 灰度
	if f.Rollout <= 0 {
		return false, nil
	}
	if f.Rollout >= 100 {
		return true, nil
	}
	uid := int64(0)
	if fc != nil {
		uid = fc.UserID
	}
	hash := crc32.ChecksumIEEE([]byte(key+":"+strconv.FormatInt(uid, 10))) % 100
	return int(hash) < f.Rollout, nil
}
