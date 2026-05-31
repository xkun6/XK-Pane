package orchestrator

import (
	"sync"
	"time"
)

// rateLimiter 是简易滑动窗口限流：windowSecs 秒内最多 maxReq 个请求（对齐 cc-pane check_rate_limit
// 的 10s≤20）。REST handler 各自调用 allow()；MCP 工具不限流（与现状一致）。
type rateLimiter struct {
	mu     sync.Mutex
	times  []time.Time
	window time.Duration
	maxReq int
	nowFn  func() time.Time // 可注入便于测试
}

func newRateLimiter(window time.Duration, maxReq int) *rateLimiter {
	return &rateLimiter{window: window, maxReq: maxReq, nowFn: time.Now}
}

// allow 在窗口内未超限时记一次并返回 true；超限返回 false。
func (r *rateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.nowFn()
	kept := r.times[:0]
	for _, t := range r.times {
		if now.Sub(t) < r.window {
			kept = append(kept, t)
		}
	}
	r.times = kept
	if len(r.times) >= r.maxReq {
		return false
	}
	r.times = append(r.times, now)
	return true
}
