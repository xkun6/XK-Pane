package terminal

import "testing"

func TestRawRingBufferSeqMonotonicAndSnapshot(t *testing.T) {
	b := newRawRingBuffer(1024)
	s1 := b.append("a")
	s2 := b.append("bb")
	s3 := b.append("ccc")
	if s1 != 1 || s2 != 2 || s3 != 3 {
		t.Fatalf("seq not monotonic from 1: got %d,%d,%d", s1, s2, s3)
	}
	data, last := b.snapshot()
	if data != "abbccc" {
		t.Fatalf("snapshot data = %q, want abbccc", data)
	}
	if last != 3 {
		t.Fatalf("snapshot lastSeq = %d, want 3", last)
	}
}

func TestRawRingBufferEvictsOldestWholeChunks(t *testing.T) {
	// maxBytes=5：写 "aaa"(3) 再 "bbb"(3) → 超限，淘汰最旧整段 "aaa"，仅留 "bbb"。
	b := newRawRingBuffer(5)
	b.append("aaa")
	b.append("bbb")
	data, last := b.snapshot()
	if data != "bbb" {
		t.Fatalf("after evict, snapshot = %q, want bbb", data)
	}
	// lastSeq 不随淘汰回退（仍为 2），客户端据此正确丢弃 live 中已覆盖的旧 chunk。
	if last != 2 {
		t.Fatalf("lastSeq after evict = %d, want 2 (monotonic, no rollback)", last)
	}
}

func TestRawRingBufferKeepsAtLeastLastChunk(t *testing.T) {
	// 单段就超 maxBytes：保留至少最后一段（否则会丢最新画面）。
	b := newRawRingBuffer(2)
	b.append("aaaaaa")
	data, last := b.snapshot()
	if data != "aaaaaa" {
		t.Fatalf("oversized single chunk dropped: snapshot = %q, want aaaaaa", data)
	}
	if last != 1 {
		t.Fatalf("lastSeq = %d, want 1", last)
	}
}

func TestRawRingBufferEmptySnapshot(t *testing.T) {
	b := newRawRingBuffer(1024)
	data, last := b.snapshot()
	if data != "" || last != 0 {
		t.Fatalf("empty snapshot = (%q,%d), want (\"\",0)", data, last)
	}
}
