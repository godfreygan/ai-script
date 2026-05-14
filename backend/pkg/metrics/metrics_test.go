package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestRecordHTTP(t *testing.T) {
	RecordHTTP("GET", "/api/v1/users", 200, 50*time.Millisecond)
	RecordHTTP("GET", "/api/v1/users", 200, 150*time.Millisecond)
	RecordHTTP("POST", "/api/v1/users", 201, 300*time.Millisecond)

	snap := HTTPRequestsTotal.Snapshot()
	// 3 label combos (method+path+status) — 前两个 status 相同所以合并为 1 条
	if len(snap) < 2 {
		t.Fatalf("expected at least 2 label combinations, got %d", len(snap))
	}
}

func TestRecordBusinessError(t *testing.T) {
	RecordBusinessError(40001)
	RecordBusinessError(40001)
	RecordBusinessError(50001)

	snap := BusinessErrorsTotal.Snapshot()
	if snap["error_code=40001"] != 2 {
		t.Fatalf("expected error_code=40001 count 2, got %d", snap["error_code=40001"])
	}
}

func TestRecordAIGeneration(t *testing.T) {
	RecordAIGeneration("script")
	RecordAIGeneration("image")
	RecordAIGeneration("image")

	snap := AIGenerationRequestsTotal.Snapshot()
	if snap["type=image"] != 2 {
		t.Fatalf("expected type=image count 2, got %d", snap["type=image"])
	}
}

func TestFormatPrometheus(t *testing.T) {
	// seed some data
	RecordHTTP("GET", "/healthz", 200, 5*time.Millisecond)
	RecordBusinessError(40001)
	RecordAIGeneration("pipeline")
	ActiveWSConnections.Set(3)

	out := FormatPrometheus()
	if out == "" {
		t.Fatal("FormatPrometheus returned empty")
	}

	checks := []string{
		"# HELP http_requests_total",
		"# TYPE http_requests_total counter",
		"http_requests_total{method=GET,path=/healthz,status=200}",
		"http_request_duration_seconds_bucket{method=GET,path=/healthz,le=0.01}",
		"http_request_duration_seconds_count{method=GET,path=/healthz}",
		"# HELP business_errors_total",
		"business_errors_total{error_code=40001}",
		"# HELP active_websocket_connections",
		"# TYPE active_websocket_connections gauge",
		"active_websocket_connections 3",
		"# HELP ai_generation_requests_total",
		"ai_generation_requests_total{type=pipeline}",
	}

	for _, s := range checks {
		if !strings.Contains(out, s) {
			t.Errorf("expected output to contain %q, got:\n%s", s, out)
		}
	}
}
