// Package metrics 提供基于 expvar 的 Prometheus 兼容指标收集。
//
// 所有指标通过 expvar 注册,在 /metrics 端点统一以 Prometheus 文本格式输出。
package metrics

import (
	"expvar"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// --- expvar 基础指标 ---

var (
	// HTTPRequestsTotal HTTP 请求总数,按 method、path、status 分类
	HTTPRequestsTotal = NewExpvarCounterMap("http_requests_total")

	// HTTPRequestDuration HTTP 请求耗时(秒),按 method、path 分类的 histogram 近似(用 bucket 计数)
	HTTPRequestDuration = NewExpvarHistogram("http_request_duration_seconds")

	// BusinessErrorsTotal 业务错误总数,按 error_code 分类
	BusinessErrorsTotal = NewExpvarCounterMap("business_errors_total")

	// ActiveWSConnections 活跃 WebSocket 连接数(gauge)
	ActiveWSConnections = expvar.NewInt("active_websocket_connections")

	// AIGenerationRequestsTotal AI 生成请求总数,按 type 分类
	AIGenerationRequestsTotal = NewExpvarCounterMap("ai_generation_requests_total")

	// CircuitBreakerStateTotal 熔断器状态变化次数,按 name、state 分类
	CircuitBreakerStateTotal = NewExpvarCounterMap("circuit_breaker_state_total")

	// QueueDepth 队列深度,按 queue 名称统计待处理任务数(gauge)
	QueueDepth = NewExpvarGaugeMap("asynq_queue_depth")

	// TaskProcessedTotal 任务处理总数,按 task_type、status 统计
	TaskProcessedTotal = NewExpvarCounterMap("asynq_task_processed_total")

	// TaskLatency 任务处理耗时 histogram(秒),按 task_type 分类
	TaskLatency = NewExpvarHistogram("asynq_task_latency_seconds")

	// WorkerRunning 当前运行的 worker 数(gauge)
	WorkerRunning = expvar.NewInt("asynq_worker_running")
)

// --- CounterMap: 支持 label 的计数器 ---

type CounterMap struct {
	mu   sync.RWMutex
	vars map[string]*expvar.Int
	name string
}

func NewExpvarCounterMap(name string) *CounterMap {
	m := &CounterMap{vars: make(map[string]*expvar.Int), name: name}
	expvar.Publish(name, expvar.Func(func() interface{} {
		return m.Snapshot()
	}))
	return m
}

func (m *CounterMap) Inc(labels map[string]string) {
	key := labelKey(labels)
	m.mu.Lock()
	v, ok := m.vars[key]
	if !ok {
		v = new(expvar.Int)
		m.vars[key] = v
	}
	v.Add(1)
	m.mu.Unlock()
}

func (m *CounterMap) Snapshot() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]int64, len(m.vars))
	for k, v := range m.vars {
		out[k] = v.Value()
	}
	return out
}

// --- GaugeMap: 支持 label 的 gauge ---

type GaugeMap struct {
	mu   sync.RWMutex
	vars map[string]*expvar.Int
	name string
}

func NewExpvarGaugeMap(name string) *GaugeMap {
	m := &GaugeMap{vars: make(map[string]*expvar.Int), name: name}
	expvar.Publish(name, expvar.Func(func() interface{} {
		return m.Snapshot()
	}))
	return m
}

func (m *GaugeMap) Set(labels map[string]string, value int64) {
	key := labelKey(labels)
	m.mu.Lock()
	v, ok := m.vars[key]
	if !ok {
		v = new(expvar.Int)
		m.vars[key] = v
	}
	v.Set(value)
	m.mu.Unlock()
}

func (m *GaugeMap) Snapshot() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]int64, len(m.vars))
	for k, v := range m.vars {
		out[k] = v.Value()
	}
	return out
}

// --- Histogram: 预定义 bucket 的近似 histogram ---

type Histogram struct {
	mu      sync.RWMutex
	buckets map[string]*expvar.Int // key = "le=0.005" 等
	sum     *expvar.Float
	count   *expvar.Int
	name    string
}

// 默认 buckets 与 Prometheus 默认一致
var defaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

func NewExpvarHistogram(name string) *Histogram {
	h := &Histogram{
		buckets: make(map[string]*expvar.Int),
		sum:     new(expvar.Float),
		count:   new(expvar.Int),
		name:    name,
	}
	for _, b := range defaultBuckets {
		h.buckets[leKey(b)] = new(expvar.Int)
	}
	// +Inf bucket
	h.buckets["le=+Inf"] = new(expvar.Int)
	expvar.Publish(name+"_bucket", expvar.Func(func() interface{} {
		return h.bucketSnapshot()
	}))
	expvar.Publish(name+"_sum", h.sum)
	expvar.Publish(name+"_count", h.count)
	return h
}

func (h *Histogram) Observe(value float64, labels map[string]string) {
	labelPart := labelKey(labels)
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, b := range defaultBuckets {
		if value <= b {
			key := labelPart + "," + leKey(b)
			v, ok := h.buckets[key]
			if !ok {
				v = new(expvar.Int)
				h.buckets[key] = v
			}
			v.Add(1)
		}
	}
	// +Inf bucket always incremented
	infKey := labelPart + ",le=+Inf"
	v, ok := h.buckets[infKey]
	if !ok {
		v = new(expvar.Int)
		h.buckets[infKey] = v
	}
	v.Add(1)

	h.sum.Add(value)
	h.count.Add(1)
}

func (h *Histogram) bucketSnapshot() map[string]int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]int64, len(h.buckets))
	for k, v := range h.buckets {
		out[k] = v.Value()
	}
	return out
}

// --- 辅助函数 ---

func leKey(b float64) string {
	return fmt.Sprintf("le=%g", b)
}

func labelKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(labels[k])
	}
	return sb.String()
}

// --- Prometheus 格式化输出 ---

// FormatPrometheus 把 expvar 中已注册的指标格式化为 Prometheus 文本格式。
func FormatPrometheus() string {
	var sb strings.Builder

	// 1. http_requests_total
	writeCounterMap(&sb, "http_requests_total", "Total number of HTTP requests.", HTTPRequestsTotal.Snapshot())

	// 2. http_request_duration_seconds (histogram)
	durBuckets := HTTPRequestDuration.bucketSnapshot()
	// group by label key (without le)
	grouped := make(map[string]map[string]int64) // labelKey -> le -> count
	for k, v := range durBuckets {
		labels, le := splitLe(k)
		if _, ok := grouped[labels]; !ok {
			grouped[labels] = make(map[string]int64)
		}
		grouped[labels][le] = v
	}
	for labels, buckets := range grouped {
		for _, b := range defaultBuckets {
			le := leKey(b)
			val := buckets[le]
			writeLine(&sb, "http_request_duration_seconds_bucket", labels, le, val)
		}
		writeLine(&sb, "http_request_duration_seconds_bucket", labels, "le=+Inf", buckets["le=+Inf"])
		writeLine(&sb, "http_request_duration_seconds_sum", labels, "", int64(HTTPRequestDuration.sum.Value()))
		writeLine(&sb, "http_request_duration_seconds_count", labels, "", HTTPRequestDuration.count.Value())
	}

	// 3. business_errors_total
	writeCounterMap(&sb, "business_errors_total", "Total number of business errors.", BusinessErrorsTotal.Snapshot())

	// 4. active_websocket_connections (gauge)
	fmt.Fprintf(&sb, "# HELP active_websocket_connections Current number of active WebSocket connections.\n")
	fmt.Fprintf(&sb, "# TYPE active_websocket_connections gauge\n")
	fmt.Fprintf(&sb, "active_websocket_connections %d\n", ActiveWSConnections.Value())

	// 5. ai_generation_requests_total
	writeCounterMap(&sb, "ai_generation_requests_total", "Total number of AI generation requests.", AIGenerationRequestsTotal.Snapshot())

	// 6. circuit_breaker_state_total
	writeCounterMap(&sb, "circuit_breaker_state_total", "Total number of circuit breaker state changes.", CircuitBreakerStateTotal.Snapshot())

	// 7. asynq_queue_depth (gauge)
	writeGaugeMap(&sb, "asynq_queue_depth", "Current number of pending tasks in asynq queues.", QueueDepth.Snapshot())

	// 8. asynq_task_processed_total
	writeCounterMap(&sb, "asynq_task_processed_total", "Total number of processed asynq tasks.", TaskProcessedTotal.Snapshot())

	// 9. asynq_task_latency_seconds (histogram)
	latBuckets := TaskLatency.bucketSnapshot()
	latGrouped := make(map[string]map[string]int64)
	for k, v := range latBuckets {
		labels, le := splitLe(k)
		if _, ok := latGrouped[labels]; !ok {
			latGrouped[labels] = make(map[string]int64)
		}
		latGrouped[labels][le] = v
	}
	for labels, buckets := range latGrouped {
		for _, b := range defaultBuckets {
			le := leKey(b)
			val := buckets[le]
			writeLine(&sb, "asynq_task_latency_seconds_bucket", labels, le, val)
		}
		writeLine(&sb, "asynq_task_latency_seconds_bucket", labels, "le=+Inf", buckets["le=+Inf"])
		writeLine(&sb, "asynq_task_latency_seconds_sum", labels, "", int64(TaskLatency.sum.Value()))
		writeLine(&sb, "asynq_task_latency_seconds_count", labels, "", TaskLatency.count.Value())
	}

	// 10. asynq_worker_running (gauge)
	fmt.Fprintf(&sb, "# HELP asynq_worker_running Current number of running asynq workers.\n")
	fmt.Fprintf(&sb, "# TYPE asynq_worker_running gauge\n")
	fmt.Fprintf(&sb, "asynq_worker_running %d\n", WorkerRunning.Value())

	return sb.String()
}

func writeCounterMap(sb *strings.Builder, name, help string, snapshot map[string]int64) {
	fmt.Fprintf(sb, "# HELP %s %s\n", name, help)
	fmt.Fprintf(sb, "# TYPE %s counter\n", name)
	for labels, val := range snapshot {
		if labels == "" {
			fmt.Fprintf(sb, "%s %d\n", name, val)
		} else {
			fmt.Fprintf(sb, "%s{%s} %d\n", name, labels, val)
		}
	}
}

func writeGaugeMap(sb *strings.Builder, name, help string, snapshot map[string]int64) {
	fmt.Fprintf(sb, "# HELP %s %s\n", name, help)
	fmt.Fprintf(sb, "# TYPE %s gauge\n", name)
	for labels, val := range snapshot {
		if labels == "" {
			fmt.Fprintf(sb, "%s %d\n", name, val)
		} else {
			fmt.Fprintf(sb, "%s{%s} %d\n", name, labels, val)
		}
	}
}

func writeLine(sb *strings.Builder, name, labels, le string, val int64) {
	if labels == "" && le == "" {
		fmt.Fprintf(sb, "%s %d\n", name, val)
		return
	}
	if le == "" {
		fmt.Fprintf(sb, "%s{%s} %d\n", name, labels, val)
		return
	}
	if labels == "" {
		fmt.Fprintf(sb, "%s{%s} %d\n", name, le, val)
		return
	}
	fmt.Fprintf(sb, "%s{%s,%s} %d\n", name, labels, le, val)
}

func splitLe(s string) (labels string, le string) {
	parts := strings.Split(s, ",")
	var labelParts []string
	for _, p := range parts {
		if strings.HasPrefix(p, "le=") {
			le = p
		} else {
			labelParts = append(labelParts, p)
		}
	}
	labels = strings.Join(labelParts, ",")
	return
}

// --- Gin 中间件便利函数 ---

// RecordHTTP 记录一次 HTTP 请求指标。
func RecordHTTP(method, path string, status int, duration time.Duration) {
	statusStr := fmt.Sprintf("%d", status)
	HTTPRequestsTotal.Inc(map[string]string{
		"method": method,
		"path":   path,
		"status": statusStr,
	})
	HTTPRequestDuration.Observe(duration.Seconds(), map[string]string{
		"method": method,
		"path":   path,
	})
}

// RecordBusinessError 记录一次业务错误。
func RecordBusinessError(code int) {
	BusinessErrorsTotal.Inc(map[string]string{
		"error_code": fmt.Sprintf("%d", code),
	})
}

// RecordAIGeneration 记录一次 AI 生成请求。
func RecordAIGeneration(genType string) {
	AIGenerationRequestsTotal.Inc(map[string]string{
		"type": genType,
	})
}

// RecordCircuitBreakerState 记录一次熔断器状态变化。
func RecordCircuitBreakerState(name, state string) {
	CircuitBreakerStateTotal.Inc(map[string]string{
		"name":  name,
		"state": state,
	})
}

// RecordTaskProcessed 记录一次任务处理结果。
func RecordTaskProcessed(taskType, status string) {
	TaskProcessedTotal.Inc(map[string]string{
		"task_type": taskType,
		"status":    status,
	})
}

// RecordTaskLatency 记录一次任务处理耗时。
func RecordTaskLatency(taskType string, duration time.Duration) {
	TaskLatency.Observe(duration.Seconds(), map[string]string{
		"task_type": taskType,
	})
}
