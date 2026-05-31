package terminal

import (
	"strings"
	"sync"
)

// 输出缓冲容量（忠实 cc-pane terminal_service：LIVE/DEAD_OUTPUT_MAX_*）。
const (
	liveOutputMaxLines = 20_000
	liveOutputMaxBytes = 20 * 1024 * 1024
	// 会话退出后缩到 dead 上限释放内存，再进 dead-buffers 供事后回看。
	deadOutputMaxLines = 20_000
	deadOutputMaxBytes = 10 * 1024 * 1024
	// partial（未遇换行的尾部）超此长度强制 flush 成一行，防进度条等持续追加撑内存。
	partialFlushBytes = 4096
)

// outputBuffer 是终端会话的输出环形缓冲（存 ANSI 已剥离的纯文本行），供 get_session_output 读取。
// 移植 cc-pane OutputBuffer：push 做 ANSI 剥离 + 换行归一 + 分行 + spinner 过滤 + 空行压缩 + 双限淘汰。
//
// 单写者（readLoop）+ 多读者（orchestrator HTTP handler）：自带 mu 串行化。
// 用 head 偏移实现 deque 摊还 O(1) 淘汰（front 淘汰只移指针，过半时压实）。
type outputBuffer struct {
	mu         sync.Mutex
	lines      []string
	head       int // 首个有效行下标；有效行 = lines[head:]
	partial    string
	maxLines   int
	maxBytes   int
	totalBytes int
}

func newOutputBuffer(maxLines, maxBytes int) *outputBuffer {
	return &outputBuffer{maxLines: maxLines, maxBytes: maxBytes}
}

// count 返回当前有效行数（持锁调用）。
func (b *outputBuffer) count() int { return len(b.lines) - b.head }

// push 追加一段原始终端输出（含 ANSI）。剥离转义、归一换行、拼接 partial 后按 \n 分行，
// 最后一段留作新的 partial；partial 超限强制 flush；超容量淘汰最旧行。
func (b *outputBuffer) push(text string) {
	clean := stripANSI(text)
	if clean == "" {
		return
	}
	normalized := strings.ReplaceAll(strings.ReplaceAll(clean, "\r\n", "\n"), "\r", "\n")

	b.mu.Lock()
	defer b.mu.Unlock()

	combined := normalized
	if b.partial != "" {
		combined = b.partial + normalized
		b.partial = ""
	}
	parts := strings.Split(combined, "\n")
	for i, part := range parts {
		if i < len(parts)-1 {
			b.pushLine(part) // 完整行（后面还有 \n）
		} else {
			b.partial = part // 最后一段 → partial
		}
	}
	if len(b.partial) > partialFlushBytes {
		line := b.partial
		b.partial = ""
		b.pushLine(line)
	}
	b.evict()
}

// pushLine 追加一行：过滤 Claude spinner 动画行；连续空行最多保留 1（持锁调用）。
func (b *outputBuffer) pushLine(line string) {
	if isSpinnerLine(line) {
		return
	}
	if strings.TrimSpace(line) == "" && b.count() > 0 {
		if strings.TrimSpace(b.lines[len(b.lines)-1]) == "" {
			return
		}
	}
	b.totalBytes += len(line)
	b.lines = append(b.lines, line)
}

// evict 淘汰最旧行直到满足行数/字节双限；head 过半时压实底层数组（持锁调用）。
func (b *outputBuffer) evict() {
	for b.count() > 0 && (b.count() > b.maxLines || b.totalBytes > b.maxBytes) {
		removed := b.lines[b.head]
		b.lines[b.head] = "" // 解引用，让字符串可被 GC
		b.head++
		b.totalBytes -= len(removed)
	}
	if b.head > 0 && b.head*2 >= len(b.lines) {
		n := copy(b.lines, b.lines[b.head:])
		b.lines = b.lines[:n]
		b.head = 0
	}
}

// shrink 把上限缩小并立即淘汰（会话退出后转 dead-buffers 时调，释放内存）。
func (b *outputBuffer) shrink(maxLines, maxBytes int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxLines = maxLines
	b.maxBytes = maxBytes
	b.evict()
}

// getRecent 返回最近 n 行的拷贝（n<=0 或 n>=总行数 → 全部）。
func (b *outputBuffer) getRecent(n int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	valid := b.lines[b.head:]
	if n <= 0 || n >= len(valid) {
		out := make([]string, len(valid))
		copy(out, valid)
		return out
	}
	out := make([]string, n)
	copy(out, valid[len(valid)-n:])
	return out
}

// stripANSI 剥离 ANSI / VT 转义序列（对齐 cc-pane strip_ansi）。状态机处理 CSI(ESC[)、
// OSC(ESC] 以 BEL 或 ST 结尾)、字符集指定(ESC( 等三字节)、其余双字节 ESC 序列；末尾孤立 ESC 丢弃。
// 非 ESC 字节原样透传（UTF-8 续字节不会是 0x1b，故按字节重建内容安全）。
func stripANSI(s string) string {
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c != 0x1b { // 非 ESC
			b.WriteByte(c)
			i++
			continue
		}
		if i+1 >= len(s) {
			break // 末尾孤立 ESC，丢弃
		}
		switch s[i+1] {
		case '[': // CSI：ESC [ ... 终止于 0x40-0x7e
			i += 2
			for i < len(s) && !(s[i] >= 0x40 && s[i] <= 0x7e) {
				i++
			}
			if i < len(s) {
				i++ // 跳过终止字节
			}
		case ']': // OSC：ESC ] ... 终止于 BEL(0x07) 或 ST(ESC \)
			i += 2
			for i < len(s) {
				if s[i] == 0x07 {
					i++
					break
				}
				if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		case '(', ')', '*', '+': // 字符集指定：ESC ( X（三字节）
			i += 3
		default: // 其余双字节 ESC 序列（ESC = / ESC > / ESC \ …）
			i += 2
		}
	}
	return b.String()
}

// spinnerWords 是 Claude Code「思考中」spinner 的随机词（忠实 cc-pane SPINNER_WORDS）。
var spinnerWords = []string{
	"Reticulating", "Swirling", "Whirlpooling", "Quantumizing", "Synthesizing",
	"Materializing", "Crystallizing", "Harmonizing", "Calibrating", "Percolating",
	"Amalgamating", "Coalescing",
}

// isSpinnerLine 判断是否为 Claude spinner 动画行（无实质内容，应被过滤）。
// 去掉前导符号/空白后，若整行 = spinner 词（可带省略号 + 计时数字），即判定为 spinner。
func isSpinnerLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	text := strings.TrimLeft(trimmed, "✻✽✶✢●·* ○◉◌")
	for _, w := range spinnerWords {
		rest, ok := strings.CutPrefix(text, w)
		if !ok {
			continue
		}
		rest = strings.TrimPrefix(rest, "…")
		rest = strings.TrimPrefix(rest, "...")
		if rest == "" {
			return true
		}
		allDigit := true
		for _, c := range rest {
			if c < '0' || c > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			return true
		}
	}
	return false
}
