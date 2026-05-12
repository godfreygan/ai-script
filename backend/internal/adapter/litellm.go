package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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
}

func NewLiteLLMAdapter(code, baseURL, apiKey, modelName string, mtype ModelType) *LiteLLMAdapter {
	return &LiteLLMAdapter{
		code:      code,
		mtype:     mtype,
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		modelName: modelName,
		client:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (a *LiteLLMAdapter) Code() string    { return a.code }
func (a *LiteLLMAdapter) Type() ModelType { return a.mtype }

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
	start := time.Now()
	switch a.mtype {
	case TypeText:
		return a.generateText(ctx, req, start)
	case TypeImage:
		return a.generateImage(ctx, req, start)
	case TypeAudio:
		return a.generateAudio(ctx, req, start)
	default:
		return nil, fmt.Errorf("adapter type %q not yet supported via litellm", a.mtype)
	}
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
	_ = json.Unmarshal(resp, &rawAny)
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
	for k, v := range req.Params {
		body[k] = v
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
	_ = json.Unmarshal(resp, &rawAny)
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
			"audio_bytes": respBody,
			"format":      "mp3",
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
		return nil, fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 512))
	}
	return respBody, nil
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
