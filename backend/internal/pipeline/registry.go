package pipeline

import (
	"github.com/hibiken/asynq"
)

// 节点类型常量
const (
	NodeScriptSplit        = "script.split"
	NodePromptGenerate     = "prompt.generate"
	NodeStoryboardGenerate = "storyboard.generate"
	NodeStyleApply         = "style.apply"
	NodeImageGenerate      = "image.generate"
	NodeImageUpload        = "image.upload"
	NodeVideoGenerate      = "video.generate"
	NodeAudioTTS           = "audio.tts"
	NodeVideoCompose       = "video.compose"
	NodeReviewSubmit       = "review.submit"
	NodeHumanApprove       = "human.approve"
	TaskPipelineRun        = "pipeline.run"
)

// HandlerRegistry 节点处理器注册中心
type HandlerRegistry struct {
	handlers map[string]asynq.HandlerFunc
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{handlers: make(map[string]asynq.HandlerFunc)}
}

func (r *HandlerRegistry) Register(name string, h asynq.HandlerFunc) {
	r.handlers[name] = h
}

func (r *HandlerRegistry) Handlers() map[string]asynq.HandlerFunc {
	out := make(map[string]asynq.HandlerFunc, len(r.handlers))
	for k, v := range r.handlers {
		out[k] = v
	}
	return out
}

// RegisterDefaults 注册默认所有节点处理器(暂用 noop,实际接 adapter)
func RegisterDefaults(r *HandlerRegistry) {
	for _, t := range []string{
		NodeScriptSplit, NodePromptGenerate, NodeStoryboardGenerate, NodeStyleApply,
		NodeImageGenerate, NodeImageUpload, NodeVideoGenerate, NodeAudioTTS,
		NodeVideoCompose, NodeReviewSubmit, NodeHumanApprove,
	} {
		t := t
		r.Register(t, NoopHandler(t))
	}
	r.Register(TaskPipelineRun, NoopHandler(TaskPipelineRun))
}
