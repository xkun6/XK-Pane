package mcpe

// logbuffer.go — MC PE 每个 run 的环形日志缓冲。忠实移植 log_buffer.rs。
//
// MC PE 日志原本只通过 emit("mc_pe:output", ...) 广播给前端，后端不留副本。MCP 工具
// get_mc_pe_logs 要让 pane 里的 Claude 读 MC 日志，就需要后端持有最近 N 行的拷贝。
//
// Go 用定长环形数组（避免 slice-as-queue 的底层数组泄漏），并自带 sync.Mutex
// —— launcher reader goroutine 写、MCP handler goroutine 读，必须并发安全
// （Rust 原版无锁，靠外层 RwLock 保护）。

import (
	"strings"
	"sync"
)

type logBuffer struct {
	mu   sync.Mutex
	buf  []string
	head int // 最旧元素下标
	size int // 有效元素个数
	cap  int
}

func newLogBuffer(cap int) *logBuffer {
	if cap < 0 {
		cap = 0
	}
	return &logBuffer{buf: make([]string, cap), cap: cap}
}

func (b *logBuffer) push(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cap == 0 {
		return
	}
	if b.size < b.cap {
		b.buf[(b.head+b.size)%b.cap] = line
		b.size++
		return
	}
	// 满：覆盖最旧、head 前移（FIFO 淘汰）
	b.buf[b.head] = line
	b.head = (b.head + 1) % b.cap
}

// tail 返回最近 n 行（不足 n 返回全部，n<=0 返回空）。
func (b *logBuffer) tail(n int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n > b.size {
		n = b.size
	}
	if n <= 0 {
		return nil
	}
	out := make([]string, n)
	start := b.size - n
	for i := 0; i < n; i++ {
		out[i] = b.buf[(b.head+start+i)%b.cap]
	}
	return out
}

func (b *logBuffer) length() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size
}

// formatLogLine 把日志字段拼成可读单行：`HH:MM:SS [Category] [LEVEL] line`，
// 缺省段按需省略。line 为空返回 ("", false)（过滤）。对齐 format_log_payload。
func formatLogLine(line, timestamp, category, level string) (string, bool) {
	if line == "" {
		return "", false
	}
	var b strings.Builder
	b.Grow(len(line) + 24)
	if timestamp != "" {
		b.WriteString(shortTS(timestamp))
		b.WriteByte(' ')
	}
	if category != "" {
		b.WriteByte('[')
		b.WriteString(category)
		b.WriteString("] ")
	}
	if level != "" {
		b.WriteByte('[')
		b.WriteString(level)
		b.WriteString("] ")
	}
	b.WriteString(line)
	return b.String(), true
}

// shortTS：`2026-05-16 16:09:57,624` → `16:09:57`；不识别就原样返回（纯字符串切割，不解析时区）。
func shortTS(ts string) string {
	space := strings.IndexByte(ts, ' ')
	if space < 0 {
		return ts
	}
	tail := ts[space+1:]
	if comma := strings.IndexByte(tail, ','); comma >= 0 {
		return tail[:comma]
	}
	return tail
}
