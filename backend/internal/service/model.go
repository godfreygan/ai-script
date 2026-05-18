package service

import (
	"context"
	"errors"

	"github.com/godfreygan/ai-script/backend/internal/adapter"
	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/crypto"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

type modelService struct {
	r        *repo.Repositories
	cipher   *crypto.Cipher
	registry *adapter.Registry
	log      *zap.Logger
}

type CreateModelInput struct {
	Code           string         `json:"code" binding:"required,min=1,max=100"`
	Name           string         `json:"name" binding:"required,min=1,max=100"`
	Type           string         `json:"type" binding:"required,oneof=text image video audio"`
	Provider       string         `json:"provider" binding:"required,min=1,max=100"`
	Endpoint       string         `json:"endpoint" binding:"required,min=1,max=500"`
	APIKey         string         `json:"api_key" binding:"omitempty,min=1,max=500"`
	ModelName      string         `json:"model_name" binding:"omitempty,min=1,max=200"` // 上游模型标识(传给 LLM 网关的 model 字段)
	DefaultParams  map[string]any `json:"default_params"`
	CapabilityTags []string       `json:"capability_tags"`
	Priority       int            `json:"priority" binding:"gte=0,lte=9999"`
	MaxQPS         int            `json:"max_qps" binding:"gte=0,lte=10000"`
	HealthCheckURL string         `json:"health_check_url" binding:"omitempty,max=500"`
}

type UpdateModelInput struct {
	Name           *string        `json:"name" binding:"omitempty,min=1,max=100"`
	Endpoint       *string        `json:"endpoint" binding:"omitempty,min=1,max=500"`
	APIKey         *string        `json:"api_key" binding:"omitempty,min=1,max=500"`
	DefaultParams  map[string]any `json:"default_params"`
	CapabilityTags []string       `json:"capability_tags"`
	Enabled        *int8          `json:"enabled" binding:"omitempty,gte=0,lte=1"`
	Priority       *int           `json:"priority" binding:"omitempty,gte=0,lte=9999"`
	MaxQPS         *int           `json:"max_qps" binding:"omitempty,gte=0,lte=10000"`
	HealthCheckURL *string        `json:"health_check_url" binding:"omitempty,max=500"`
}

func (s *modelService) List(ctx context.Context, q *repo.ListModelsQuery) ([]model.Model, int64, error) {
	return s.r.Model.List(ctx, q)
}

func (s *modelService) Get(ctx context.Context, id int64) (*model.Model, error) {
	m, err := s.r.Model.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	// 修复: 返回前清空加密字段,防止任何意外泄露
	m.APIKeyEncrypted = nil
	return m, nil
}

func (s *modelService) Create(ctx context.Context, in *CreateModelInput) (*model.Model, error) {
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

func (s *modelService) Update(ctx context.Context, id int64, in *UpdateModelInput) (*model.Model, error) {
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

func (s *modelService) Delete(ctx context.Context, id int64) error {
	return s.r.Model.Delete(ctx, id)
}

// Healthcheck 调一次模型适配器,把结果写回 last_health_status
func (s *modelService) Healthcheck(ctx context.Context, id int64) (bool, error) {
	m, err := s.r.Model.Get(ctx, id)
	if err != nil {
		return false, errcode.ErrNotFound
	}
	apiKey, err := s.decryptKey(m)
	if err != nil {
		return false, err
	}
	a := s.getOrBuildAdapter(m, apiKey)
	hcCtx, cancel := context.WithTimeout(ctx, getTimeout("TIMEOUT_MODEL_HEALTH", 30))
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
func (s *modelService) GetAdapter(ctx context.Context, id int64) (adapter.Adapter, *model.Model, error) {
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

func (s *modelService) decryptKey(m *model.Model) (string, error) {
	if len(m.APIKeyEncrypted) == 0 {
		return "", nil
	}
	plain, err := s.cipher.Decrypt(m.APIKeyEncrypted)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *modelService) registerAdapter(m *model.Model, plainKey string) {
	if s.registry == nil {
		return
	}
	s.registry.Register(s.buildAdapter(m, plainKey))
}

func (s *modelService) getOrBuildAdapter(m *model.Model, plainKey string) adapter.Adapter {
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

func (s *modelService) buildAdapter(m *model.Model, plainKey string) adapter.Adapter {
	modelName := readModelName(m)
	mtype := adapter.ModelType(m.Type)

	switch mtype {
	case adapter.TypeVideo:
		return adapter.NewVideoAdapter(m.Code, m.Endpoint, plainKey, modelName)
	default:
		// text / image / audio 统一走 LiteLLM OpenAI-compatible 网关
		return adapter.NewLiteLLMAdapter(m.Code, m.Endpoint, plainKey, modelName, mtype)
	}
}

// =============== 启动时把所有 enabled 模型注册到 Registry ===============

func (s *modelService) LoadAllAdapters(ctx context.Context) error {
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
