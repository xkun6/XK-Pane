package localhistory

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestCompressBlobs(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)

	big := bytes.Repeat([]byte("abcd"), 500) // 2000 字节，可压性强
	v, _ := s.SaveVersion(ctx, "big.txt", big, false, "main", 0)
	if _, err := s.SaveVersion(ctx, "small.txt", []byte("tiny"), false, "main", 0); err != nil {
		t.Fatalf("save small: %v", err)
	}

	bigPath := s.blobPath(v.Hash)
	if fi, _ := os.Stat(bigPath); fi == nil || fi.Size() != 2000 {
		t.Fatalf("pre-compress big blob size != 2000")
	}

	n, err := s.CompressBlobs(ctx)
	if err != nil {
		t.Fatalf("CompressBlobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("compressed = %d, want 1 (big only; small <512B 跳过)", n)
	}

	// big blob 现在更小且带 gzip 魔数。
	raw, _ := os.ReadFile(bigPath)
	if len(raw) >= 2000 || len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("big blob not gzip-compressed: size=%d", len(raw))
	}

	// 透明解压：内容仍是原始 2000 字节。
	got, err := s.GetVersionContent(ctx, "big.txt", v.ID)
	if err != nil || !bytes.Equal(got, big) {
		t.Fatalf("roundtrip after compress failed: err=%v len=%d want %d", err, len(got), len(big))
	}

	// 幂等：再压一次为 0（big 已压、small 太小）。
	if n2, _ := s.CompressBlobs(ctx); n2 != 0 {
		t.Errorf("second compress = %d, want 0", n2)
	}
}
