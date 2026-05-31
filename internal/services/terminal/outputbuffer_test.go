package terminal

import (
	"strings"
	"testing"
)

// push 基本分行：完整行入 lines，末尾无换行段留作 partial（不可见于 getRecent 直到补换行）。
func TestOutputBufferPushBasicLines(t *testing.T) {
	b := newOutputBuffer(100, 1<<20)
	b.push("line1\nline2\npartial")
	got := b.getRecent(0)
	if len(got) != 2 || got[0] != "line1" || got[1] != "line2" {
		t.Fatalf("lines = %v, want [line1 line2]", got)
	}
	// 补上换行后 partial 转完整行
	b.push("-more\n")
	got = b.getRecent(0)
	if len(got) != 3 || got[2] != "partial-more" {
		t.Fatalf("after completing partial: %v, want 3rd = partial-more", got)
	}
}

// \r\n 与单独 \r 归一为 \n。
func TestOutputBufferNormalizesCR(t *testing.T) {
	b := newOutputBuffer(100, 1<<20)
	b.push("a\r\nb\rc\n")
	got := b.getRecent(0)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("lines = %v, want [a b c]", got)
	}
}

// ANSI 转义被剥离（颜色 CSI + 光标移动）。
func TestOutputBufferStripsANSI(t *testing.T) {
	b := newOutputBuffer(100, 1<<20)
	b.push("\x1b[31mred\x1b[0m\x1b[2Ktext\n")
	got := b.getRecent(0)
	if len(got) != 1 || got[0] != "redtext" {
		t.Fatalf("lines = %v, want [redtext]", got)
	}
}

// spinner 行被过滤，实质行保留。忠实 cc-pane：仅「词 / 词… / 词+纯数字」算 spinner。
func TestOutputBufferFiltersSpinner(t *testing.T) {
	b := newOutputBuffer(100, 1<<20)
	b.push("✻ Synthesizing…\n")
	b.push("● Coalescing\n")
	b.push("real output\n")
	got := b.getRecent(0)
	if len(got) != 1 || got[0] != "real output" {
		t.Fatalf("lines = %v, want only [real output]", got)
	}
}

// 连续空行最多保留 1。
func TestOutputBufferCompressesBlankLines(t *testing.T) {
	b := newOutputBuffer(100, 1<<20)
	b.push("a\n\n\n\nb\n")
	got := b.getRecent(0)
	if len(got) != 3 || got[0] != "a" || got[1] != "" || got[2] != "b" {
		t.Fatalf("lines = %v, want [a, '', b]", got)
	}
}

// 行数超限淘汰最旧行。
func TestOutputBufferEvictsByLines(t *testing.T) {
	b := newOutputBuffer(3, 1<<20)
	for _, s := range []string{"1\n", "2\n", "3\n", "4\n", "5\n"} {
		b.push(s)
	}
	got := b.getRecent(0)
	if len(got) != 3 || got[0] != "3" || got[2] != "5" {
		t.Fatalf("lines = %v, want [3 4 5]", got)
	}
}

// 字节超限淘汰最旧行（每行 ~5 字节，上限 12 字节 → 仅留最后 2 行）。
func TestOutputBufferEvictsByBytes(t *testing.T) {
	b := newOutputBuffer(1000, 12)
	for _, s := range []string{"aaaaa\n", "bbbbb\n", "ccccc\n"} {
		b.push(s)
	}
	got := b.getRecent(0)
	if len(got) != 2 || got[0] != "bbbbb" || got[1] != "ccccc" {
		t.Fatalf("lines = %v, want [bbbbb ccccc]", got)
	}
}

// getRecent(n) 取最近 n 行。
func TestOutputBufferGetRecentN(t *testing.T) {
	b := newOutputBuffer(100, 1<<20)
	b.push("1\n2\n3\n4\n5\n")
	got := b.getRecent(2)
	if len(got) != 2 || got[0] != "4" || got[1] != "5" {
		t.Fatalf("getRecent(2) = %v, want [4 5]", got)
	}
}

// partial 超 4KB 强制 flush 成一行（防进度条撑内存）。
func TestOutputBufferFlushesLongPartial(t *testing.T) {
	b := newOutputBuffer(100, 1<<20)
	b.push(strings.Repeat("x", 5000)) // 无换行
	got := b.getRecent(0)
	if len(got) != 1 || len(got[0]) != 5000 {
		t.Fatalf("long partial not flushed: %d lines", len(got))
	}
}

// shrink 缩上限并立即淘汰。
func TestOutputBufferShrink(t *testing.T) {
	b := newOutputBuffer(100, 1<<20)
	b.push("1\n2\n3\n4\n5\n")
	b.shrink(2, 1<<20)
	got := b.getRecent(0)
	if len(got) != 2 || got[0] != "4" || got[1] != "5" {
		t.Fatalf("after shrink: %v, want [4 5]", got)
	}
}

func TestStripANSI(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"a\x1b[2Kb", "ab"},            // CSI 擦除
		{"x\x1b]0;title\x07y", "xy"},   // OSC + BEL
		{"x\x1b]0;title\x1b\\y", "xy"}, // OSC + ST
		{"u\x1b(Bv", "uv"},             // 字符集指定（三字节）
		{"end\x1b", "end"},             // 末尾孤立 ESC 丢弃
		{"中\x1b[1m文", "中文"},            // 多字节内容穿插转义
	}
	for _, c := range cases {
		if got := stripANSI(c.in); got != c.want {
			t.Errorf("stripANSI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsSpinnerLine(t *testing.T) {
	// 忠实 cc-pane（窄）：词 / 词… / 词+纯数字 算 spinner；带括号计时（"… (3s)"）不算。
	spin := []string{"✻ Synthesizing…", "● Calibrating...", "  Percolating", "Coalescing123"}
	notSpin := []string{"", "Building project", "✻ done", "Synthesizing the report", "✻ Calibrating… (12s)"}
	for _, s := range spin {
		if !isSpinnerLine(s) {
			t.Errorf("isSpinnerLine(%q) = false, want true", s)
		}
	}
	for _, s := range notSpin {
		if isSpinnerLine(s) {
			t.Errorf("isSpinnerLine(%q) = true, want false", s)
		}
	}
}
