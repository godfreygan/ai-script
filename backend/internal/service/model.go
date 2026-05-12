package service

import (
	"context"
	"errors"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/adapter"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/crypto"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

type ModelService struct {
	r        *repo.Repositories
	cipher   *crypto.Cipher
	registry *adapter.Registry
	log      *zap.Logger
}

type CreateModelInput struct {
	Code           string         `json:"code" binding:"required"`
	Name           string         `json:"name" binding:"required"`
	Type           string         `json:"type" binding:"required,oneof=text image video audio"`
	Provider       string         `json:"provider" binding:"required"`
	Endpoint       string         `json:"endpoint" binding:"required"`
	APIKey         string         `json:"api_key"`
	ModelName      string         `json:"model_name"` // 上游模型标识(传给 LLM 网关的 model 字段)
	DefaultParams  map[string]any `json:"default_params"`
	CapabilityTags []string       `json:"capability_tags"`
	Priority       int            `json:"priority"`
	MaxQPS         int            `json:"max_qps"`
	HealthCheckURL string         `json:"health_check_url"`
}

type UpdateModelInput struct {
	Name           *string        `json:"name"`
	Endpoint       *string        `json:"endpoint"`
	APIKey         *string        `json:"api_key"`
	DefaultParams  map[string]any `json:"default_params"`
	CapabilityTags []string       `json:"capability_tags"`
	Enabled        *int8          `json:"enabled"`
	Priority       *int           `json:"priority"`
	MaxQPS         *int           `json:"max_qps"`
	HealthCheckURL *string        `json:"health_check_url"`
}

func (s *ModelService) List(ctx context.Context, q *repo.ListModelsQuery) ([]model.Model, int64, error) {
	return s.r.Model.List(ctx, q)
}

func (s *ModelService) Get(ctx context.Context, id int64) (*model.Model, error) {
	m, err := s.r.Model.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return m, nil
}

func (s *ModelService) Create(ctx context.Context, in *CreateModelInput) (*model.Model, error) {
	if _, err := s.r.Model.GetByCode(ctx, in.Code); err == nil {
		return nil, errcode.ErrConflict
	}
	m := &model.Model{
		Code:           in.Code,
		Name:           in.Name,
		Type:           in.Type,
		Provider:       in.Provider,
		Endpoint:       in.Endpoint,
		DefaultParams:  toJSON(mergeModelName(in.DefaultParams, in.ModelName)),
		CapabilityTags: toJSON(in.CapabilityTags),
		Enabled:        1,
		Priority:       in.Priority,
		MaxQPS:         in.MaxQPS,
		HealthCheckURL: in.HealthCheckURL,
	}
	if in.APIKey != "" {
		ct, err := s.cipher.Encrypt([]byte(in.APIKey))
		if err != nil {
			return nil, err
		}
		m.APIKeyEncrypted = ct
	}
	if err := s.r.Model.Create(ctx, m); err != nil {
		return nil, err
	}
	s.registerAdapter(m, in.APIKey)
	return m, nil
}

func (s *ModelService) Update(ctx context.Context, id int64, in *UpdateModelInput) (*model.Model, error) {
	m, err := s.r.Model.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	if in.Name != nil {
		m.Name = *in.Name
	}
	if in.Endpoint != nil {
		m.Endpoint = *in.Endpoint
	}
	if in.DefaultParams != nil {
		m.DefaultParams = toJSON(in.DefaultParams)
	}
	if in.CapabilityTags != nil {
		m.CapabilityTags = toJSON(in.CapabilityTags)
	}
	if in.Enabled != nil {
		m.Enabled = *in.Enabled
	}
	if in.Priority != nil {
		m.Priority = *in.Priority
	}
	if in.MaxQPS != nil {
		m.MaxQPS = *in.MaxQPS
	}
	if in.HealthCheckURL != nil {
		m.HealthCheckURL = *in.HealthCheckURL
	}
	if err := s.r.Model.Update(ctx, m); err != nil {
		return nil, err
	}
	if in.APIKey != nil && *in.APIKey != "" {
		ct, err := s.cipher.Encrypt([]byte(*in.APIKey))
		if err != nil {
			return nil, err
		}
		if err := s.r.Model.UpdateAPIKey(ctx, id, ct); err != nil {
			return nil, err
		}
		s.registerAdapter(m, *in.APIKey)
	}
	return m, nil
}

func (s *ModelService) Delete(ctx context.Context, id int64) error {
	return s.r.Model.Delete(ctx, id)
}

// Healthcheck 调一次模型适配器,把结果写回 last_health_status
func (s *ModelService) Healthcheck(ctx context.Context, id int64) (bool, error) {
	m, err := s.r.Model.Get(ctx, id)
	if err != nil {
		return false, errcode.ErrNotFound
	}
	apiKey, err := s.decryptKey(m)
	if err != nil {
		return false, err
	}
	a := s.getOrBuildAdapter(m, apiKey)
	hcCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := a.Healthcheck(hcCtx); err != nil {
		// 用原始 ctx 写库,避免 hcCtx 超时被一并取消导致状态写不回
		_ = s.r.Model.UpdateHealth(ctx, id, 2)
		return false, err
	}
	_ = s.r.Model.UpdateHealth(ctx, id, 1)
	return true, nil
}

// GetAdapter 获取一个模型的 adapter,惰性构建并缓存
func (s *ModelService) GetAdapter(ctx context.Context, id int64) (adapter.Adapter, *model.Model, error) {
	m, err := s.r.Model.Get(ctx, id)
	if err != nil {
		return nil, nil, errcode.ErrNotFound
	}
	if m.Enabled != 1 {
		return nil, m, errors.New("model disabled")
	}
	apiKey, err := s.decryptKey(m)
	if err != nil {
		return nil, m, err
	}
	return s.getOrBuildAdapter(m, apiKey), m, nil
}

func (s *ModelService) decryptKey(m *model.Model) (string, error) {
	if len(m.APIKeyEncrypted) == 0 {
		return "", nil
	}
	plain, err := s.cipher.Decrypt(m.APIKeyEncrypted)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *ModelService) registerAdapter(m *model.Model, plainKey string) {
	if s.registry == nil {
		return
	}
	s.registry.Register(s.buildAdapter(m, plainKey))
}

func (s *ModelService) getOrBuildAdapter(m *model.Model, plainKey string) adapter.Adapter {
	if s.registry == nil {
		return s.buildAdapter(m, plainKey)
	}
	if a, err := s.registry.Get(m.Code); err == nil {
		return a
	}
	a := s.buildAdapter(m, plainKey)
	s.registry.Register(a)
	return a
}

func (s *ModelService) buildAdapter(m *model.Model, plainKey string) adapter.Adapter {
	// 当前 MVP 用统一 LiteLLM HTTP 适配器,后续可在此根据 m.Provider 切换具体实现
	modelName := readModelName(m)
	return adapter.NewLiteLLMAdapter(m.Code, m.Endpoint, plainKey, modelName, adapter.ModelType(m.Type))
}

// =============== 启动时把所有 enabled 模型注册到 Registry ===============

func (s *ModelService) LoadAllAdapters(ctx context.Context) error {
	list, err := s.r.Model.ListAllEnabled(ctx)
	if err != nil {
		return err
	}
	for i := range list {
		m := list[i]
		apiKey, err := s.decryptKey(&m)
		if err != nil {
			s.log.Warn("decrypt api_key failed", zap.String("code", m.Code), zap.Error(err))
			continue
		}
		s.registry.Register(s.buildAdapter(&m, apiKey))
	}
	return nil
}
