// Package circuitbreaker 提供简易熔断器实现。
//
// 三态模型：Closed（正常）-> Open（熔断）-> Half-Open（探测）-> Closed/Open
// 默认配置：失败阈值 5 次，熔断时间 30 秒，半开探测请求 1 次。
package circuitbreaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State 熔断器状态
type State int32

const (
	StateClosed   State = iota // 正常关闭，请求通过
	StateOpen                  // 熔断打开，请求快速失败
	StateHalfOpen              // 半开状态，允许探测请求
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	// ErrOpen 熔断器打开时返回的错误
	ErrOpen = errors.New("circuit breaker is open")
)

// Config 熔断器配置
type Config struct {
	// FailureThreshold 触发熔断的连续失败阈值
	FailureThreshold uint32
	// OpenDuration 熔断持续时间
	OpenDuration time.Duration
	// HalfOpenMaxCalls 半开状态允许的最大探测请求数
	HalfOpenMaxCalls uint32
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		OpenDuration:     30 * time.Second,
		HalfOpenMaxCalls: 1,
	}
}

// CircuitBreaker 简易熔断器
type CircuitBreaker struct {
	name string
	cfg  Config

	state      int32 // atomic State
	failures   int32 // atomic 连续失败计数
	successes  int32 // atomic 半开成功计数（用于半开状态）
	lastFailAt int64 // atomic unix nano

	onStateChange func(name string, from, to State)
	mu            sync.RWMutex
}

// New 创建熔断器实例
func New(name string, cfg Config) *CircuitBreaker {
	return &CircuitBreaker{
		name: name,
		cfg:  cfg,
	}
}

// NewWithDefault 使用默认配置创建熔断器
func NewWithDefault(name string) *CircuitBreaker {
	return New(name, DefaultConfig())
}

// State 返回当前状态（线程安全）
func (cb *CircuitBreaker) State() State {
	return State(atomic.LoadInt32(&cb.state))
}

// Call 执行受保护的调用。
// 若熔断器处于 Open 状态，直接返回 ErrOpen。
// 否则执行 fn，并根据结果更新状态。
func (cb *CircuitBreaker) Call(fn func() error) error {
	state := cb.currentState()
	if state == StateOpen {
		return ErrOpen
	}

	err := fn()
	cb.recordResult(state, err)
	return err
}

// currentState 计算当前状态（考虑超时自动进入半开）
func (cb *CircuitBreaker) currentState() State {
	state := State(atomic.LoadInt32(&cb.state))
	if state == StateOpen {
		lastFail := atomic.LoadInt64(&cb.lastFailAt)
		if time.Since(time.Unix(0, lastFail)) >= cb.cfg.OpenDuration {
			// 尝试从 Open -> Half-Open
			if atomic.CompareAndSwapInt32(&cb.state, int32(StateOpen), int32(StateHalfOpen)) {
				atomic.StoreInt32(&cb.failures, 0)
				atomic.StoreInt32(&cb.successes, 0)
				cb.notifyStateChange(StateOpen, StateHalfOpen)
			}
			return State(atomic.LoadInt32(&cb.state))
		}
	}
	return state
}

// recordResult 根据调用结果更新状态
func (cb *CircuitBreaker) recordResult(before State, err error) {
	if err != nil {
		cb.onFailure(before)
		return
	}
	cb.onSuccess(before)
}

func (cb *CircuitBreaker) onFailure(before State) {
	switch before {
	case StateClosed:
		failures := atomic.AddInt32(&cb.failures, 1)
		if uint32(failures) >= cb.cfg.FailureThreshold {
			// 尝试进入 Open
			if atomic.CompareAndSwapInt32(&cb.state, int32(StateClosed), int32(StateOpen)) {
				atomic.StoreInt64(&cb.lastFailAt, time.Now().UnixNano())
				cb.notifyStateChange(StateClosed, StateOpen)
			}
		}
	case StateHalfOpen:
		// 半开状态失败，立即回到 Open
		if atomic.CompareAndSwapInt32(&cb.state, int32(StateHalfOpen), int32(StateOpen)) {
			atomic.StoreInt64(&cb.lastFailAt, time.Now().UnixNano())
			atomic.StoreInt32(&cb.failures, 0)
			cb.notifyStateChange(StateHalfOpen, StateOpen)
		}
	}
}

func (cb *CircuitBreaker) onSuccess(before State) {
	switch before {
	case StateClosed:
		// 成功时重置失败计数
		atomic.StoreInt32(&cb.failures, 0)
	case StateHalfOpen:
		successes := atomic.AddInt32(&cb.successes, 1)
		if uint32(successes) >= cb.cfg.HalfOpenMaxCalls {
			// 探测成功次数达到阈值，关闭熔断器
			if atomic.CompareAndSwapInt32(&cb.state, int32(StateHalfOpen), int32(StateClosed)) {
				atomic.StoreInt32(&cb.failures, 0)
				atomic.StoreInt32(&cb.successes, 0)
				cb.notifyStateChange(StateHalfOpen, StateClosed)
			}
		}
	}
}

// SetOnStateChange 设置状态变更回调（非线程安全，建议在初始化时设置）
func (cb *CircuitBreaker) SetOnStateChange(fn func(name string, from, to State)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

func (cb *CircuitBreaker) notifyStateChange(from, to State) {
	cb.mu.RLock()
	fn := cb.onStateChange
	cb.mu.RUnlock()
	if fn != nil {
		fn(cb.name, from, to)
	}
}

// Name 返回熔断器名称
func (cb *CircuitBreaker) Name() string {
	return cb.name
}
