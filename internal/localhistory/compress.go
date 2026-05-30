package localhistory

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"

	"xkpane/internal/utils/filex"
)

// CompressBlobs 把 blobs 目录下「未压缩且 ≥512B」的 blob 文件 gzip 压缩（仅当压缩后更小才替换，
// 原子写避免并发读到半包），返回被压缩的文件数。对齐 cc-pane compress_blobs。
//
// 透明性：blob 文件名仍是**原始内容**的 SHA256，压缩只改文件内容；GetVersionContent 读时按
// gzip 魔数自动解压，故压缩对去重 / 读取 / 二次保存均透明（SaveVersion 写新 blob 仍写原始字节，
// 压缩是事后单独触发的维护操作）。
func (s *Store) CompressBlobs(ctx context.Context) (int, error) {
	entries, err := os.ReadDir(s.blobs)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	compressed := 0
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return compressed, err
		}
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".db-shm") {
			continue
		}
		path := filepath.Join(s.blobs, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
			continue // 已是 gzip
		}
		if len(raw) < 512 {
			continue // 太小，压缩收益不值
		}

		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(raw); err != nil {
			_ = gw.Close()
			continue
		}
		if err := gw.Close(); err != nil {
			continue
		}
		comp := buf.Bytes()
		if len(comp) < len(raw) { // 仅当确实更小才替换
			if err := filex.WriteAtomic(path, comp, 0o644); err != nil {
				continue
			}
			compressed++
		}
	}
	return compressed, nil
}
