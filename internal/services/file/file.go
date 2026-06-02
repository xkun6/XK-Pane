// Package file 提供「文件浏览器」后端能力（Wails service）：列目录树、读/写文件、
// 在系统资源管理器中定位、以及「最近打开目录」登记。
//
// 设计要点：
//   - 文件 IO 一律走 internal/utils/filex（WriteAtomic 原子写 / ReadLimit 限长读），
//     绝不裸用 os.WriteFile / io.ReadAll（工具层铁律）。
//   - 外部进程（RevealInOS）走 internal/utils/procx，平台实现拆 reveal_<os>.go。
//   - 路径原样使用，绝不规范化（D7 铁律）；去重靠 recent_directories 的 uniqueIndex。
//   - ListDir 不记录访问（导航噪声大）；只有显式 RecordRecent 才登记「最近目录」。
package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/filex"

	"gorm.io/gorm"
)

var (
	errEmptyPath = errors.New("empty path")
	errNotDir    = errors.New("path is not a directory")
	errIsDir     = errors.New("path is a directory")
)

// maxReadBytes 是 ReadFile 的单文件上限（编辑器场景，超大文件不适合内联编辑）。
const maxReadBytes = 8 << 20 // 8MB

// FileEntry 是目录下一个条目的展示用 DTO（os.DirEntry 是接口，无法经 Wails 序列化）。
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`    // filepath.Join(dir, name)，原样拼接
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`    // 字节；目录为 0
	ModTime int64  `json:"modTime"` // unix 毫秒
}

// FileContent 是 ReadFile 的返回：文本内容 + 元信息。二进制文件不返内容（IsBinary=true）。
type FileContent struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Size     int64  `json:"size"`
	IsBinary bool   `json:"isBinary"`
}

type Service struct {
	holder *storage.Holder
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// ListDir 列出目录的直接子项（单层，懒加载由前端逐层展开驱动）。
// 目录在前、文件在后，各自按名称大小写不敏感升序。
func (s *Service) ListDir(ctx context.Context, path string) ([]FileEntry, error) {
	if path == "" {
		return nil, errEmptyPath
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errNotDir
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		var size, mod int64
		if info, ierr := e.Info(); ierr == nil {
			size = info.Size()
			mod = info.ModTime().UnixMilli()
		}
		out = append(out, FileEntry{
			Name:    e.Name(),
			Path:    filepath.Join(path, e.Name()),
			IsDir:   e.IsDir(),
			Size:    size,
			ModTime: mod,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir // 目录在前
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// ReadFile 读取文件文本内容（上限 maxReadBytes）。二进制文件（含 NUL 字节）只返元信息。
func (s *Service) ReadFile(ctx context.Context, path string) (FileContent, error) {
	if path == "" {
		return FileContent{}, errEmptyPath
	}
	fi, err := os.Stat(path)
	if err != nil {
		return FileContent{}, err
	}
	if fi.IsDir() {
		return FileContent{}, errIsDir
	}
	data, err := filex.ReadLimit(path, maxReadBytes)
	if err != nil {
		return FileContent{}, err
	}
	if isBinary(data) {
		return FileContent{Path: path, Size: fi.Size(), IsBinary: true}, nil
	}
	return FileContent{Path: path, Content: string(data), Size: fi.Size()}, nil
}

// WriteFile 把内容原子写回文件（走 filex.WriteAtomic，自动建中间目录）。
func (s *Service) WriteFile(ctx context.Context, path string, content string) error {
	if path == "" {
		return errEmptyPath
	}
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return errIsDir
	}
	return filex.WriteAtomic(path, []byte(content), 0o644)
}

// RevealInOS 在系统文件管理器中定位 path（目录则打开，文件则选中）。平台实现见 reveal_<os>.go。
func (s *Service) RevealInOS(ctx context.Context, path string) error {
	if path == "" {
		return errEmptyPath
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return revealInOS(ctx, path)
}

// RecordRecent 登记一个「文件浏览器打开的根目录」（去重 upsert：已存在则计数+1 并刷新时间）。
func (s *Service) RecordRecent(ctx context.Context, path string) error {
	if path == "" {
		return errEmptyPath
	}
	now := time.Now()
	db := s.db().WithContext(ctx)
	var rd storage.RecentDirectory
	err := db.Where("path = ?", path).First(&rd).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&storage.RecentDirectory{Path: path, OpenCount: 1, LastOpenedAt: now}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&rd).Updates(map[string]any{
		"open_count":     gorm.Expr("open_count + 1"),
		"last_opened_at": now,
	}).Error
}

// ListRecent 返回最近打开的目录（按最后打开时间倒序）。limit<=0 或过大时取默认 20。
func (s *Service) ListRecent(ctx context.Context, limit int) ([]storage.RecentDirectory, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	out := make([]storage.RecentDirectory, 0)
	if err := s.db().WithContext(ctx).Order("last_opened_at DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// isBinary 用「首 8KB 是否含 NUL 字节」粗判二进制，避免把二进制塞进文本编辑器。
func isBinary(data []byte) bool {
	n := len(data)
	if n > 8000 {
		n = 8000
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}
