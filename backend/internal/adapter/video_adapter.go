package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godfreygan/ai-script/backend/pkg/circuitbreaker"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
)

// VideoAdapter 视频生成模型适配器(Kling / Runway / Pika 风格)。
//
// 视频生成 API 通常是异步模式:
//  1. POST /v1/video/generations 提交任务,返回 task_id
//  2. GET  /v1/video/generations/{task_id} 轮询结果,直到 status=completed
//
// 本实现兼容两类网关:
//   - 同步返回:直接返回 video_url(如某些 OpenAI-compatible 代理)
//   - 异步返回:通过 task_id 轮询(如 Kling / Runway 原生 API)
type VideoAdapter struct {
	code      string
	baseURL   string
	apiKey    string
	modelName string
	client    *http.Client
	cb        *circuitbreaker.CircuitBreaker
}

var videoCbRegistry sync.Map // key=endpoint, value=*circuitbreaker.CircuitBreaker

var videoStateChangeRecorder atomic.Value // func(name, state string)

// SetVideoCircuitBreakerStateRecorder 设置熔断器状态变更记录回调。
func SetVideoCircuitBreakerStateRecorder(fn func(name, state string)) {
	videoStateChangeRecorder.Store(fn)
}

func recordVideoCircuitBreakerState(name, state string) {
	if v := videoStateChangeRecorder.Load(); v != nil {
		v.(func(string, string))(name, state)
	}
}

func getVideoCircuitBreaker(endpoint string) *circuitbreaker.CircuitBreaker {
	if v, ok := videoCbRegistry.Load(endpoint); ok {
		return v.(*circuitbreaker.CircuitBreaker)
	}
	cb := circuitbreaker.NewWithDefault(endpoint)
	cb.SetOnStateChange(func(name string, from, to circuitbreaker.State) {
		recordVideoCircuitBreakerState(name, to.String())
	})
	actual, _ := videoCbRegistry.LoadOrStore(endpoint, cb)
	return actual.(*circuitbreaker.CircuitBreaker)
}

// NewVideoAdapter 创建视频模型适配器。
//
// endpoint 为网关地址,例如:
//   - https://api.klingai.com
//   - https://api.runwayml.com
//   - https://api.pika.art
//
// 若 endpoint 已含 /v1 路径,则直接使用;否则自动补 /v1。
func NewVideoAdapter(code, baseURL, apiKey, modelName string) *VideoAdapter {
	ep := strings.TrimRight(baseURL, "/")
	return &VideoAdapter{
		code:      code,
		baseURL:   ep,
		apiKey:    apiKey,
		modelName: modelName,
		client:    &http.Client{Timeout: 5 * time.Minute},
		cb:        getVideoCircuitBreaker(ep),
	}
}

func (a *VideoAdapter) Code() string    { return a.code }
func (a *VideoAdapter) Type() ModelType { return TypeVideo }

// videoGenerationRequest 提交视频生成任务的请求体
type videoGenerationRequest struct {
	Model  string         `json:"model"`
	Prompt string         `json:"prompt"`
	Inputs []string       `json:"inputs,omitempty"` // 输入图片 URL(图生视频)
	Params map[string]any `json:"params,omitempty"` // 透传额外参数
}

// videoGenerationResponse 同步/异步通用响应
type videoGenerationResponse struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"` // pending / processing / completed / failed
	VideoURL string `json:"video_url"`
	Error    *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
	// 部分厂商返回在 data 嵌套中
	Data *struct {
		TaskID   string `json:"task_id"`
		Status   string `json:"status"`
		VideoURL string `json:"video_url"`
		Videos   []struct {
			URL string `json:"url"`
		} `json:"videos"`
	} `json:"data"`
}

// videoTaskResponse 查询任务状态的响应
type videoTaskResponse struct {
	Status   string `json:"status"`
	VideoURL string `json:"video_url"`
	Error    *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
	Data *struct {
		Status   string `json:"status"`
		VideoURL string `json:"video_url"`
		Videos   []struct {
			URL string `json:"url"`
		} `json:"videos"`
	} `json:"data"`
}

// Generate 提交视频生成任务并等待完成。
//
// 流程:
//  1. 提交生成请求 -> 获取 task_id
//  2. 轮询任务状态(默认 5s 间隔,最多 120 次 = 10 分钟)
//  3. 完成后返回 video_url
func (a *VideoAdapter) Generate(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()
	var resp *Response
	var genErr error

	err := a.cb.Call(func() error {
		resp, genErr = a.generateWithPolling(ctx, req, start)
		return genErr
	})
	if err == circuitbreaker.ErrOpen {
		return nil, errcode.ErrUpstreamModel.WithMsg("上游视频模型服务熔断，请稍后重试")
	}
	if err != nil {
		return nil, err
	}
	return resp, genErr
}

func (a *VideoAdapter) generateWithPolling(ctx context.Context, req *Request, start time.Time) (*Response, error) {
	// 1. 提交任务
	body := videoGenerationRequest{
		Model:  a.modelName,
		Prompt: req.Prompt,
		Inputs: req.Inputs,
		Params: make(map[string]any),
	}
	// 透传额外参数
	maps.Copy(body.Params, req.Params)

	submitURL := a.fullURL("/video/generations")
	if req.Params["submit_path"] != nil {
		if sp, ok := req.Params["submit_path"].(string); ok && sp != "" {
			submitURL = a.fullURL(sp)
		}
	}

	submitBody, _ := json.Marshal(body)
	submitReq, err := http.NewRequestWithContext(ctx, "POST", submitURL, bytes.NewReader(submitBody))
	if err != nil {
		return nil, fmt.Errorf("build submit request: %w", err)
	}
	if a.apiKey != "" {
		submitReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	submitReq.Header.Set("Content-Type", "application/json")

	submitResp, err := a.client.Do(submitReq)
	if err != nil {
		return nil, fmt.Errorf("submit video task: %w", err)
	}
	defer submitResp.Body.Close()

	submitData, _ := io.ReadAll(submitResp.Body)
	if submitResp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("submit video task HTTP %d: %s", submitResp.StatusCode, truncateVideo(string(submitData), 512))
	}

	var submitResult videoGenerationResponse
	if err := json.Unmarshal(submitData, &submitResult); err != nil {
		return nil, fmt.Errorf("decode submit response: %w", err)
	}
	if submitResult.Error != nil {
		return nil, fmt.Errorf("upstream submit error: %s", submitResult.Error.Message)
	}

	// 同步直接返回 video_url 的情况
	videoURL := extractVideoURL(&submitResult)
	if videoURL != "" {
		return &Response{
			VideoURLs:  []string{videoURL},
			Units:      1,
			DurationMs: int(time.Since(start).Milliseconds()),
			Raw: map[string]any{
				"video_url": videoURL,
				"sync":      true,
			},
		}, nil
	}

	// 2. 异步轮询
	taskID := submitResult.TaskID
	if taskID == "" && submitResult.Data != nil {
		taskID = submitResult.Data.TaskID
	}
	if taskID == "" {
		return nil, fmt.Errorf("no task_id returned and no video_url in sync response")
	}

	pollURL := a.fullURL("/video/generations/" + taskID)
	if req.Params["poll_path"] != nil {
		if pp, ok := req.Params["poll_path"].(string); ok && pp != "" {
			pollURL = a.fullURL(fmt.Sprintf(pp, taskID))
		}
	}

	pollInterval := 5 * time.Second
	if req.Params["poll_interval_sec"] != nil {
		if sec, ok := req.Params["poll_interval_sec"].(float64); ok && sec > 0 {
			pollInterval = time.Duration(sec) * time.Second
		}
	}
	maxPolls := 120 // 最多轮询 120 次 = 10 分钟(按 5s 间隔)
	if req.Params["max_polls"] != nil {
		if mp, ok := req.Params["max_polls"].(float64); ok && mp > 0 {
			maxPolls = int(mp)
		}
	}

	for i := 0; i < maxPolls; i++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("video generation cancelled: %w", ctx.Err())
		case <-time.After(pollInterval):
		}

		pollReq, err := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build poll request: %w", err)
		}
		if a.apiKey != "" {
			pollReq.Header.Set("Authorization", "Bearer "+a.apiKey)
		}

		pollResp, err := a.client.Do(pollReq)
		if err != nil {
			return nil, fmt.Errorf("poll task status: %w", err)
		}

		pollData, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()

		if pollResp.StatusCode/100 != 2 {
			return nil, fmt.Errorf("poll task HTTP %d: %s", pollResp.StatusCode, truncateVideo(string(pollData), 512))
		}

		var pollResult videoTaskResponse
		if err := json.Unmarshal(pollData, &pollResult); err != nil {
			return nil, fmt.Errorf("decode poll response: %w", err)
		}
		if pollResult.Error != nil {
			return nil, fmt.Errorf("upstream poll error: %s", pollResult.Error.Message)
		}

		status := pollResult.Status
		if status == "" && pollResult.Data != nil {
			status = pollResult.Data.Status
		}

		switch status {
		case "completed", "success", "succeeded", "done":
			videoURL = extractVideoURLFromTask(&pollResult)
			if videoURL == "" {
				return nil, fmt.Errorf("task completed but no video_url returned")
			}
			return &Response{
				VideoURLs:  []string{videoURL},
				Units:      1,
				DurationMs: int(time.Since(start).Milliseconds()),
				Raw: map[string]any{
					"video_url": videoURL,
					"task_id":   taskID,
					"sync":      false,
				},
			}, nil
		case "failed", "error", "cancelled":
			errMsg := "task failed"
			if pollResult.Error != nil {
				errMsg = pollResult.Error.Message
			}
			return nil, fmt.Errorf("video generation %s: %s", status, errMsg)
		case "pending", "processing", "in_progress", "queued":
			// 继续轮询
			continue
		default:
			// 未知状态,继续轮询
			continue
		}
	}

	return nil, fmt.Errorf("video generation polling timeout after %d attempts", maxPolls)
}

func extractVideoURL(r *videoGenerationResponse) string {
	if r.VideoURL != "" {
		return r.VideoURL
	}
	if r.Data != nil {
		if r.Data.VideoURL != "" {
			return r.Data.VideoURL
		}
		if len(r.Data.Videos) > 0 && r.Data.Videos[0].URL != "" {
			return r.Data.Videos[0].URL
		}
	}
	return ""
}

func extractVideoURLFromTask(r *videoTaskResponse) string {
	if r.VideoURL != "" {
		return r.VideoURL
	}
	if r.Data != nil {
		if r.Data.VideoURL != "" {
			return r.Data.VideoURL
		}
		if len(r.Data.Videos) > 0 && r.Data.Videos[0].URL != "" {
			return r.Data.Videos[0].URL
		}
	}
	return ""
}

// Healthcheck 探测视频模型服务健康状态。
// 优先使用 /models 端点(与 LiteLLM 兼容),若返回非 2xx 则降级为 GET /
func (a *VideoAdapter) Healthcheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	url := a.fullURL("/models")
	r, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if a.apiKey != "" {
		r.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	resp, err := a.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 == 2 {
		return nil
	}

	// 降级:尝试根路径
	url = a.fullURL("/")
	r, _ = http.NewRequestWithContext(ctx, "GET", url, nil)
	if a.apiKey != "" {
		r.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	resp2, err := a.client.Do(r)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode/100 != 2 {
		return fmt.Errorf("unhealthy: HTTP %d", resp2.StatusCode)
	}
	return nil
}

// fullURL 拼接调用 URL — 如果 baseURL 已含 /v1,直接拼;否则补一个 /v1
func (a *VideoAdapter) fullURL(path string) string {
	base := a.baseURL
	if strings.HasSuffix(base, "/v1") || strings.Contains(base, "/v1/") {
		return base + path
	}
	return base + "/v1" + path
}

func truncateVideo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
