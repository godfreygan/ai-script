package adapter

import (
	"context"
	"errors"
	"sync"
)

// ModelType 模型类型
type ModelType string

const (
	// TypeText 文本模型(对话/补全)
	TypeText ModelType = "text"
	// TypeImage 图像生成模型
	TypeImage ModelType = "image"
	// TypeVideo 视频生成模型
	TypeVideo ModelType = "video"
	// TypeAudio 音频/TTS 模型
	TypeAudio ModelType = "audio"
)

// Request 通用请求
type Request struct {
	Prompt    string         `json:"prompt"`
	NegPrompt string         `json:"neg_prompt"`
	Inputs    []string       `json:"inputs"` // 输入图片/音频 URL
	Params    map[string]any `json:"params"`
}

// Response 通用响应
type Response struct {
	Texts        []string       `json:"texts"`
	ImageURLs    []string       `json:"image_urls"`
	VideoURLs    []string       `json:"video_urls"`
	AudioURLs    []string       `json:"audio_urls"`
	InputTokens  int            `json:"input_tokens"`
	OutputTokens int            `json:"output_tokens"`
	Units        int            `json:"units"`
	DurationMs   int            `json:"duration_ms"`
	Raw          map[string]any `json:"raw"`
}

// Adapter 模型适配器接口
type Adapter interface {
	Code() string
	Type() ModelType
	Generate(ctx context.Context, req *Request) (*Response, error)
	Healthcheck(ctx context.Context) error
}

// Registry 适配器注册中心
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

func NewRegistry() *Registry { return &Registry{adapters: make(map[string]Adapter)} }

func (r *Registry) Register(a Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[a.Code()] = a
}

func (r *Registry) Get(code string) (Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if a, ok := r.adapters[code]; ok {
		return a, nil
	}
	return nil, errors.New("adapter not found: " + code)
}

// 示例:OpenAI / Anthropic / Aliyun / Volcengine 各自实现 Adapter 接口
// 推荐做法:用统一网关(LiteLLM / One-API)做中转,Adapter 只需一两个 HTTP 实现
