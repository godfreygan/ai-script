package adapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godfreygan/ai-script/backend/pkg/circuitbreaker"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"
)

// cacheEntry 带 TTL 的缓存项
type cacheEntry struct {
	resp     *Response
	cachedAt time.Time
}

// LiteLLMAdapter LiteLLM / One-API 风格的 OpenAI 兼容 HTTP 适配器
//
// 文本模型走 /chat/completions
// 图像模型走 /images/generations(部分网关)
// 视频模型暂不在 OpenAI 协议中,留待具体厂商 adapter 实现
type LiteLLMAdapter struct {
	code      string
	mtype     ModelType
	baseURL   string
	apiKey    string
	modelName string
	client    *http.Client
	cb        *circuitbreaker.CircuitBreaker
	sf        singleflight.Group
	cache     *lru.Cache[string, *cacheEntry]
	cacheTTL  time.Duration
}

var cbRegistry sync.Map // key=endpoint, value=*circuitbreaker.CircuitBreaker

// stateChangeRecorder 用于注入 metrics 记录回调，避免 adapter 直接依赖 metrics 包造成循环依赖。
// 由 metrics 包或初始化代码通过 SetCircuitBreakerStateRecorder 设置。
var stateChangeRecorder atomic.Value // func(name, state string)

// SetCircuitBreakerStateRecorder 设置熔断器状态变更记录回调。
func SetCircuitBreakerStateRecorder(fn func(name, state string)) {
	stateChangeRecorder.Store(fn)
}

func recordCircuitBreakerState(name, state string) {
	if v := stateChangeRecorder.Load(); v != nil {
		v.(func(string, string))(name, state)
	}
}

func getCircuitBreaker(endpoint string) *circuitbreaker.CircuitBreaker {
	if v, ok := cbRegistry.Load(endpoint); ok {
		return v.(*circuitbreaker.CircuitBreaker)
	}
	cb := circuitbreaker.NewWithDefault(endpoint)
	cb.SetOnStateChange(func(name string, from, to circuitbreaker.State) {
		recordCircuitBreakerState(name, to.String())
	})
	actual, _ := cbRegistry.LoadOrStore(endpoint, cb)
	return actual.(*circuitbreaker.CircuitBreaker)
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Minute,
		Transport: &http.Transport{
			MaxConnsPerHost:     20,
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func NewLiteLLMAdapter(code, baseURL, apiKey, modelName string, mtype ModelType) *LiteLLMAdapter {
	ep := strings.TrimRight(baseURL, "/")
	cache, _ := lru.New[string, *cacheEntry](128)
	return &LiteLLMAdapter{
		code:      code,
		mtype:     mtype,
		baseURL:   ep,
		apiKey:    apiKey,
		modelName: modelName,
		client:    newHTTPClient(),
		cb:        getCircuitBreaker(ep),
		cache:     cache,
		cacheTTL:  5 * time.Second,
	}
}

func (a *LiteLLMAdapter) Code() string    { return a.code }
func (a *LiteLLMAdapter) Type() ModelType { return a.mtype }

// cacheKey 根据模型 code、prompt 及关键参数生成确定性缓存键。
func (a *LiteLLMAdapter) cacheKey(req *Request) string {
	h := sha256.New()
	h.Write([]byte(a.code))
	h.Write([]byte{0})
	h.Write([]byte(req.Prompt))
	h.Write([]byte{0})
	h.Write([]byte(a.modelName))
	h.Write([]byte{0})
	// 把 temperature / max_tokens / top_p / system 等影响结果的关键参数纳入
	keys := make([]string, 0, len(req.Params))
	for k := range req.Params {
		switch k {
		case "temperature", "max_tokens", "top_p", "system", "presence_penalty", "frequency_penalty":
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, _ := json.Marshal(req.Params[k])
		h.Write([]byte(k))
		h.Write([]byte{1})
		h.Write(v)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (a *LiteLLMAdapter) getCached(key string) *Response {
	if a.cache == nil {
		return nil
	}
	if ent, ok := a.cache.Get(key); ok && time.Since(ent.cachedAt) < a.cacheTTL {
		return ent.resp
	}
	return nil
}

func (a *LiteLLMAdapter) setCached(key string, resp *Response) {
	if a.cache == nil {
		return
	}
	a.cache.Add(key, &cacheEntry{resp: resp, cachedAt: time.Now()})
}

// chatResponse 用于解析 OpenAI 兼容 /chat/completions 响应
type chatResponse struct {
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// imageResponse 用于解析 OpenAI 兼容 /images/generations 响应
type imageResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *LiteLLMAdapter) Generate(ctx context.Context, req *Request) (*Response, error) {
	key := a.cacheKey(req)

	// 1. 先读本地 LRU 缓存
	if cached := a.getCached(key); cached != nil {
		return cached, nil
	}

	// 2. singleflight 合并并发相同请求
	// 使用独立 ctx 防止首个调用者 cancel 传染所有等待协程
	v, err, _ := a.sf.Do(key, func() (any, error) {
		// 再次检查缓存（可能前面协程已写入）
		if cached := a.getCached(key); cached != nil {
			return cached, nil
		}

		start := time.Now()
		var resp *Response
		var genErr error
		cbErr := a.cb.Call(func() error {
			// 独立超时 ctx，避免外部调用者 cancel 导致后续等待协程失败
			sfCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			switch a.mtype {
			case TypeText:
				resp, genErr = a.generateText(sfCtx, req, start)
			case TypeImage:
				resp, genErr = a.generateImage(sfCtx, req, start)
			case TypeAudio:
				resp, genErr = a.generateAudio(sfCtx, req, start)
			default:
				genErr = fmt.Errorf("adapter type %q not yet supported via litellm", a.mtype)
			}
			return genErr
		})
		if cbErr == circuitbreaker.ErrOpen {
			return nil, errcode.ErrUpstreamModel.WithMsg("上游模型服务熔断，请稍后重试")
		}
		if genErr != nil {
			return nil, genErr
		}
		// 仅缓存成功的响应
		a.setCached(key, resp)
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Response), nil
}

func (a *LiteLLMAdapter) generateText(ctx context.Context, req *Request, start time.Time) (*Response, error) {
	body := map[string]any{
		"model": a.modelName,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
	}
	// 把 system prompt / temperature 等参数透传
	for k, v := range req.Params {
		if k == "system" {
			if s, ok := v.(string); ok && s != "" {
				msgs := body["messages"].([]map[string]string)
				body["messages"] = append([]map[string]string{{"role": "system", "content": s}}, msgs...)
				continue
			}
		}
		body[k] = v
	}
	resp, err := a.doJSON(ctx, "/chat/completions", body)
	if err != nil {
		return nil, err
	}
	var cr chatResponse
	if err := json.Unmarshal(resp, &cr); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	if cr.Error != nil {
		return nil, fmt.Errorf("upstream: %s", cr.Error.Message)
	}
	texts := make([]string, 0, len(cr.Choices))
	for _, c := range cr.Choices {
		texts = append(texts, c.Message.Content)
	}
	var rawAny map[string]any
	if err := json.Unmarshal(resp, &rawAny); err != nil {
		rawAny = nil
	}
	return &Response{
		Texts:        texts,
		InputTokens:  cr.Usage.PromptTokens,
		OutputTokens: cr.Usage.CompletionTokens,
		DurationMs:   int(time.Since(start).Milliseconds()),
		Raw:          rawAny,
	}, nil
}

func (a *LiteLLMAdapter) generateImage(ctx context.Context, req *Request, start time.Time) (*Response, error) {
	body := map[string]any{
		"model":  a.modelName,
		"prompt": req.Prompt,
		"n":      1,
		"size":   "1024x1024",
	}
	if req.NegPrompt != "" {
		body["negative_prompt"] = req.NegPrompt
	}
	// 白名单透传，禁止覆盖核心字段
	allowed := map[string]bool{"n": true, "size": true, "quality": true, "style": true, "response_format": true, "user": true}
	for k, v := range req.Params {
		if allowed[k] {
			body[k] = v
		}
	}
	resp, err := a.doJSON(ctx, "/images/generations", body)
	if err != nil {
		return nil, err
	}
	var ir imageResponse
	if err := json.Unmarshal(resp, &ir); err != nil {
		return nil, fmt.Errorf("decode image response: %w", err)
	}
	if ir.Error != nil {
		return nil, fmt.Errorf("upstream: %s", ir.Error.Message)
	}
	urls := make([]string, 0, len(ir.Data))
	for _, d := range ir.Data {
		if d.URL != "" {
			urls = append(urls, d.URL)
		}
	}
	var rawAny map[string]any
	if err := json.Unmarshal(resp, &rawAny); err != nil {
		rawAny = nil
	}
	return &Response{
		ImageURLs:  urls,
		Units:      len(urls),
		DurationMs: int(time.Since(start).Milliseconds()),
		Raw:        rawAny,
	}, nil
}

// generateAudio 走 OpenAI 兼容的 /audio/speech (TTS) 协议。
//
// 请求 JSON 例:
//
//	{"model":"tts-1", "input":"你好世界", "voice":"alloy", "response_format":"mp3"}
//
// 响应直接为音频二进制(audio/mpeg 或 audio/wav)。
// 这里把字节通过 Raw["audio_bytes"] 透出,由上层 service 写入对象存储后得到可访问的 URL。
func (a *LiteLLMAdapter) generateAudio(ctx context.Context, req *Request, start time.Time) (*Response, error) {
	body := map[string]any{
		"model":           a.modelName,
		"input":           req.Prompt,
		"voice":           "alloy",
		"response_format": "mp3",
	}
	for k, v := range req.Params {
		body[k] = v
	}
	buf, _ := json.Marshal(body)
	url := a.fullURL("/audio/speech")
	r, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	if a.apiKey != "" {
		r.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		// 部分网关失败仍返回 JSON,尝试解析 error 字段
		var errResp struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		if errResp.Error != nil {
			return nil, fmt.Errorf("upstream: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 256))
	}
	// 估算时长:粗略按文本长度估(约 200ms/字),后续上层可重新 ffprobe
	chars := len([]rune(req.Prompt))
	estMs := chars * 180
	return &Response{
		Units:      1,
		DurationMs: int(time.Since(start).Milliseconds()),
		Raw: map[string]any{
			"audio_bytes":     respBody,
			"format":          "mp3",
			"est_duration_ms": estMs,
		},
	}, nil
}

// doJSON 执行 OpenAI 兼容的 POST JSON 请求
func (a *LiteLLMAdapter) doJSON(ctx context.Context, path string, body any) ([]byte, error) {
	buf, _ := json.Marshal(body)
	url := a.fullURL(path)
	r, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	if a.apiKey != "" {
		r.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, a.formatHTTPError(resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// formatHTTPError 根据 HTTP 状态码生成用户友好的错误信息
func (a *LiteLLMAdapter) formatHTTPError(statusCode int, body string) error {
	// 尝试解析上游返回的 JSON 错误
	var errResp struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(body), &errResp) == nil && errResp.Error != nil {
		return fmt.Errorf("upstream: %s", errResp.Error.Message)
	}

	// 根据状态码提供友好提示
	switch statusCode {
	case http.StatusNotFound: // 404
		return fmt.Errorf("模型接口不存在 (404): 请检查模型网关地址 %s 是否正确", a.baseURL)
	case http.StatusUnauthorized: // 401
		return fmt.Errorf("模型认证失败 (401): 请检查 API Key 配置")
	case http.StatusForbidden: // 403
		return fmt.Errorf("模型访问被拒绝 (403): 请检查 API Key 权限")
	case http.StatusTooManyRequests: // 429
		return fmt.Errorf("模型请求过于频繁 (429): 请稍后重试")
	case http.StatusInternalServerError: // 500
		return fmt.Errorf("模型服务内部错误 (500): %s", truncate(body, 200))
	case http.StatusBadGateway: // 502
		return fmt.Errorf("模型网关错误 (502): 上游服务不可达")
	case http.StatusServiceUnavailable: // 503
		return fmt.Errorf("模型服务不可用 (503): 请稍后重试")
	default:
		return fmt.Errorf("upstream HTTP %d: %s", statusCode, truncate(body, 512))
	}
}

// fullURL 拼接调用 URL — 如果 baseURL 已含 /v1,直接拼;否则补一个 /v1
func (a *LiteLLMAdapter) fullURL(path string) string {
	base := a.baseURL
	if strings.HasSuffix(base, "/v1") || strings.Contains(base, "/v1/") {
		return base + path
	}
	return base + "/v1" + path
}

func (a *LiteLLMAdapter) Healthcheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
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
	_, _ = io.Copy(io.Discard, resp.Body) // 排空 Body 以复用 TCP 连接
	if resp.StatusCode/100 != 2 {
		return errors.New("unhealthy: HTTP " + fmt.Sprint(resp.StatusCode))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
