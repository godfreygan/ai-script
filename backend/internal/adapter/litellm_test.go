package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiteLLMAdapter_Generate_singleflight_dedup(t *testing.T) {
	var callCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// 模拟轻微延迟，让并发请求有机会重叠
		time.Sleep(50 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": "hello from singleflight",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     3,
				"completion_tokens": 5,
				"total_tokens":      8,
			},
		})
	}))
	defer ts.Close()

	adapter := NewLiteLLMAdapter("gpt-test", ts.URL, "fake-key", "gpt-4", TypeText)
	adapter.cacheTTL = 5 * time.Second

	req := &Request{
		Prompt: "say hello",
		Params: map[string]any{
			"temperature": 0.7,
			"max_tokens":  100,
		},
	}

	const n = 10
	results := make(chan *Response, n)
	errors := make(chan error, n)

	ctx := context.Background()
	for i := 0; i < n; i++ {
		go func() {
			resp, err := adapter.Generate(ctx, req)
			if err != nil {
				errors <- err
				return
			}
			results <- resp
		}()
	}

	for i := 0; i < n; i++ {
		select {
		case err := <-errors:
			t.Fatalf("unexpected error: %v", err)
		case resp := <-results:
			require.NotNil(t, resp)
			assert.Equal(t, []string{"hello from singleflight"}, resp.Texts)
			assert.Equal(t, 3, resp.InputTokens)
			assert.Equal(t, 5, resp.OutputTokens)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for response")
		}
	}

	// 并发 10 个相同请求，上游只应被调用 1 次
	assert.Equal(t, int32(1), callCount.Load(), "upstream should be called exactly once due to singleflight dedup")
}

func TestLiteLLMAdapter_Generate_cache_hit(t *testing.T) {
	var callCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": fmt.Sprintf("call %d", callCount.Load()),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}))
	defer ts.Close()

	adapter := NewLiteLLMAdapter("gpt-test", ts.URL, "fake-key", "gpt-4", TypeText)
	adapter.cacheTTL = 5 * time.Second

	req := &Request{Prompt: "cache me", Params: map[string]any{}}
	ctx := context.Background()

	// 第一次调用，走上游
	resp1, err := adapter.Generate(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, []string{"call 1"}, resp1.Texts)
	assert.Equal(t, int32(1), callCount.Load())

	// 第二次调用，应命中缓存，不再请求上游
	resp2, err := adapter.Generate(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, []string{"call 1"}, resp2.Texts)
	assert.Equal(t, int32(1), callCount.Load(), "second call should hit cache")
}

func TestLiteLLMAdapter_Generate_cache_expired(t *testing.T) {
	var callCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": fmt.Sprintf("call %d", callCount.Load()),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}))
	defer ts.Close()

	adapter := NewLiteLLMAdapter("gpt-test", ts.URL, "fake-key", "gpt-4", TypeText)
	adapter.cacheTTL = 100 * time.Millisecond // 短 TTL

	req := &Request{Prompt: "expire me", Params: map[string]any{}}
	ctx := context.Background()

	_, err := adapter.Generate(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load())

	// 等待缓存过期
	time.Sleep(200 * time.Millisecond)

	_, err = adapter.Generate(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int32(2), callCount.Load(), "cache expired should trigger second upstream call")
}

func TestLiteLLMAdapter_cacheKey_deterministic(t *testing.T) {
	a := NewLiteLLMAdapter("m1", "http://localhost", "k", "gpt-4", TypeText)

	req1 := &Request{
		Prompt: "hello",
		Params: map[string]any{"temperature": 0.5, "max_tokens": 100},
	}
	req2 := &Request{
		Prompt: "hello",
		Params: map[string]any{"max_tokens": 100, "temperature": 0.5},
	}
	req3 := &Request{
		Prompt: "hello",
		Params: map[string]any{"temperature": 0.6, "max_tokens": 100},
	}

	assert.Equal(t, a.cacheKey(req1), a.cacheKey(req2), "same params different order should yield same key")
	assert.NotEqual(t, a.cacheKey(req1), a.cacheKey(req3), "different param value should yield different key")
}

func TestLiteLLMAdapter_Generate_error_not_cached(t *testing.T) {
	var callCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "boom"}})
	}))
	defer ts.Close()

	adapter := NewLiteLLMAdapter("gpt-test", ts.URL, "fake-key", "gpt-4", TypeText)
	adapter.cacheTTL = 5 * time.Second

	req := &Request{Prompt: "fail me", Params: map[string]any{}}
	ctx := context.Background()

	_, err1 := adapter.Generate(ctx, req)
	require.Error(t, err1)
	assert.Equal(t, int32(1), callCount.Load())

	_, err2 := adapter.Generate(ctx, req)
	require.Error(t, err2)
	assert.Equal(t, int32(2), callCount.Load(), "error responses should not be cached")
}
