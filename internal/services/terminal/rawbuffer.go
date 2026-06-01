package terminal

import (
	"strings"
	"sync"
)

// rawReplayMaxBytes 是原始 ANSI 重放缓冲的字节上限。够装最近一两屏全屏重绘 + 部分回滚，
// 又不至于撑内存（每活跃会话最多 +512KB）。超限按「整段」淘汰最旧 chunk。
const rawReplayMaxBytes = 512 * 1024

// rawChunk 是一段已 emit 的原始终端输出（含 ANSI），带单调 seq —— 与对应 terminal:output
// 事件的 seq 一致，客户端据此对 replay 与后续 live 去重（不漏不重）。
type rawChunk struct {
	seq  uint64
	data string
}

// rawRingBuffer 保存最近若干段**原始 ANSI** 输出，供 attach 时重放重建 xterm 画面。
//
// 与 outputBuffer 的区别：outputBuffer 存的是 stripANSI 后的纯文本行（给 get_session_output
// 读，重建不了带颜色/光标的终端画面）；本缓冲存原样字节流，xterm.write 后即还原画面。
//
// 单写者（batchLoop flush）+ 多读者（replay handler）：自带 mu 串行化。按字节上限淘汰最旧
// 整段（chunk 边界，与 emit 粒度一致）。lastSeq 单调递增、不随淘汰回退（= 历史最大 seq），
// 即使缓冲被淘汰空，客户端仍能据 lastSeq 正确丢弃 live 中已覆盖的旧 chunk。
type rawRingBuffer struct {
	mu         sync.Mutex
	chunks     []rawChunk
	totalBytes int
	maxBytes   int
	lastSeq    uint64
}

func newRawRingBuffer(maxBytes int) *rawRingBuffer {
	return &rawRingBuffer{maxBytes: maxBytes}
}

// append 追加一段原始输出，分配并返回其 seq。超字节上限则从前淘汰最旧整段（始终保留至少
// 最后一段，避免单段超限时清空 → 那样会丢掉最新画面）。
func (b *rawRingBuffer) append(data string) uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastSeq++
	seq := b.lastSeq
	b.chunks = append(b.chunks, rawChunk{seq: seq, data: data})
	b.totalBytes += len(data)
	for b.totalBytes > b.maxBytes && len(b.chunks) > 1 {
		b.totalBytes -= len(b.chunks[0].data)
		b.chunks[0] = rawChunk{} // 解引用，让被淘汰段的字符串可被 GC
		b.chunks = b.chunks[1:]
	}
	return seq
}

// snapshot 返回当前缓冲拼接的原始数据 + 历史最大 seq（lastSeq）。持锁原子读，保证「拼到的
// 数据」与「返回的 lastSeq」一致：此后 emit 的 chunk seq 必 > lastSeq（客户端写之；缓冲内
// 已含的 chunk seq 必 ≤ lastSeq，客户端丢之）。
func (b *rawRingBuffer) snapshot() (string, uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.chunks) == 0 {
		return "", b.lastSeq
	}
	var sb strings.Builder
	sb.Grow(b.totalBytes)
	for _, c := range b.chunks {
		sb.WriteString(c.data)
	}
	return sb.String(), b.lastSeq
}
