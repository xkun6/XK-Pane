package file

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"xkpane/internal/utils/filex"
)

// 全文内容搜索（grep 兜底式：纯 Go 遍历 + 逐行不区分大小写匹配）。无索引，靠三重预算防卡死：
// 单文件大小上限 / 命中条数上限 / 时间上限。跳过高噪声目录与二进制文件。
// 注：mac 版 fanbox 还有 Spotlight(mdfind) 秒级 + PDF/OCR，Windows 无对应物，故只做 grep 路径。

const (
	maxContentMatches  = 200
	maxSearchFileBytes = 512 << 10 // 512KB：超过不进全文搜索（大文件多为生成物/媒体）
	searchDeadline     = 4 * time.Second
	maxLineTextRunes   = 200 // 命中行预览截断
)

// searchIgnoreDirs 与 filewatch 同款高噪声目录黑名单（不进搜索）。
var searchIgnoreDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true,
	"target": true, ".xkpanes": true, "vendor": true, "bin": true,
	".task": true, ".next": true, ".cache": true, "out": true,
	"coverage": true, ".idea": true, ".gradle": true,
}

// ContentMatch 是一条全文命中（文件 + 行号 + 行预览）。
type ContentMatch struct {
	Path string `json:"path"` // 绝对路径
	Rel  string `json:"rel"`  // 相对 root 的展示路径（正斜杠）
	Line int    `json:"line"` // 1-based 行号
	Text string `json:"text"` // 命中行内容（trim + 截断）
}

// SearchContent 在 root 下递归全文搜索 query（不区分大小写，子串匹配）。
// 入参不规范化（D7）；命中达上限 / 超时即停。query 为空返回 nil。
func (s *Service) SearchContent(_ context.Context, root, query string) ([]ContentMatch, error) {
	if root == "" {
		return nil, errEmptyPath
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	lower := strings.ToLower(q)
	out := make([]ContentMatch, 0, 64)
	start := time.Now()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil // 无法访问的项跳过
		}
		if len(out) >= maxContentMatches || time.Since(start) > searchDeadline {
			return filepath.SkipAll
		}
		if d.IsDir() {
			if path != root && searchIgnoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil || info.Size() == 0 || info.Size() > maxSearchFileBytes {
			return nil
		}
		data, rerr := filex.ReadLimit(path, maxSearchFileBytes)
		if rerr != nil || isBinary(data) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		lineNo := 0
		for _, line := range strings.Split(string(data), "\n") {
			lineNo++
			if strings.Contains(strings.ToLower(line), lower) {
				out = append(out, ContentMatch{
					Path: path,
					Rel:  rel,
					Line: lineNo,
					Text: clipRunes(strings.TrimSpace(line), maxLineTextRunes),
				})
				if len(out) >= maxContentMatches {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	return out, nil
}

const (
	maxListedFiles = 8000            // Ctrl+P 文件清单条数上限（超大仓库截断）
	listDeadline   = 4 * time.Second // 遍历时间上限
)

// ListedFile 是 Ctrl+P 快速打开的一条文件项（绝对路径 + 相对展示路径）。
type ListedFile struct {
	Path string `json:"path"` // 绝对路径（打开用）
	Rel  string `json:"rel"`  // 相对 root 正斜杠路径（展示 + 模糊匹配）
}

// ListFiles 递归列出 root 下所有文件（供 Ctrl+P 快速打开模糊匹配）。跳过高噪声目录；
// 条数 / 时间双预算防超大仓库卡死。入参不规范化（D7）；不读文件内容，故无二进制过滤。
func (s *Service) ListFiles(_ context.Context, root string) ([]ListedFile, error) {
	if root == "" {
		return nil, errEmptyPath
	}
	out := make([]ListedFile, 0, 256)
	start := time.Now()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil // 无法访问的项跳过
		}
		if len(out) >= maxListedFiles || time.Since(start) > listDeadline {
			return filepath.SkipAll
		}
		if d.IsDir() {
			if path != root && searchIgnoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		out = append(out, ListedFile{Path: path, Rel: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return out, err
	}
	return out, nil
}

// clipRunes 把 s 截断到至多 n 个 rune（避免超长行撑爆传输 / UI）。
func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
