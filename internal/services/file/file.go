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
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/filex"

	"gorm.io/gorm"
)

var (
	errEmptyPath   = errors.New("empty path")
	errNotDir      = errors.New("path is not a directory")
	errIsDir       = errors.New("path is a directory")
	errEmptyName   = errors.New("empty name")
	errNameTooLong = errors.New("name too long")
	errBadName     = errors.New("invalid name")
	errExists      = errors.New("target already exists")
	errTooLarge    = errors.New("file too large to preview")
)

// maxReadBytes 是 ReadFile 的单文件上限（编辑器场景，超大文件不适合内联编辑）。
const maxReadBytes = 8 << 20 // 8MB

// maxPreviewBytes 是 ReadFileBase64 的上限（图片等就地预览，超大不内联，让前端降级为系统打开）。
const maxPreviewBytes = 16 << 20 // 16MB

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

// ReadFileBase64 读取文件并返回 data URI（用于图片等非文本就地预览）。
// 超过 maxPreviewBytes 报错（让前端降级为「用系统打开」）。入参不规范化（D7）。
func (s *Service) ReadFileBase64(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", errEmptyPath
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", errIsDir
	}
	if fi.Size() > maxPreviewBytes {
		return "", errTooLarge
	}
	data, err := filex.ReadLimit(path, maxPreviewBytes)
	if err != nil {
		return "", err
	}
	return "data:" + mimeForPath(path) + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// CreateFile 在 dir 下新建空文件 name（O_EXCL：已存在则报错，绝不覆盖）。返回新文件绝对路径。
func (s *Service) CreateFile(ctx context.Context, dir, name string) (string, error) {
	target, err := newChildPath(dir, name)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	_ = f.Close()
	return target, nil
}

// CreateDir 在 dir 下新建子目录 name（已存在则报错）。返回新目录绝对路径。
func (s *Service) CreateDir(ctx context.Context, dir, name string) (string, error) {
	target, err := newChildPath(dir, name)
	if err != nil {
		return "", err
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		return "", err
	}
	return target, nil
}

// Rename 把 path 重命名为同目录下的 newName。返回新路径。
// 仅当不是「纯大小写改名」时才查同名冲突（大小写不敏感卷上 Foo→foo 合法）。
func (s *Service) Rename(ctx context.Context, path, newName string) (string, error) {
	if path == "" {
		return "", errEmptyPath
	}
	if err := validName(newName); err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	target := filepath.Join(filepath.Dir(path), newName)
	if !strings.EqualFold(filepath.Base(path), newName) {
		if _, err := os.Stat(target); err == nil {
			return "", errExists
		}
	}
	if err := os.Rename(path, target); err != nil {
		return "", err
	}
	return target, nil
}

// Move 把 src 移动到目标目录 dstDir 下（保持文件名，同名自动加 -2/-3 序号防覆盖）。
// 同卷走 os.Rename（快），跨卷（*os.LinkError，EXDEV）回退 copy + 删除。返回最终目标路径。
func (s *Service) Move(ctx context.Context, src, dstDir string) (string, error) {
	if src == "" || dstDir == "" {
		return "", errEmptyPath
	}
	sfi, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	dfi, err := os.Stat(dstDir)
	if err != nil {
		return "", err
	}
	if !dfi.IsDir() {
		return "", errNotDir
	}
	target := uniqueTarget(filepath.Join(dstDir, filepath.Base(src)))
	if err := os.Rename(src, target); err != nil {
		var le *os.LinkError
		if errors.As(err, &le) { // 跨卷：copy + 删除
			if cerr := copyPath(src, target, sfi.IsDir()); cerr != nil {
				return "", cerr
			}
			if rerr := os.RemoveAll(src); rerr != nil {
				return "", rerr
			}
			return target, nil
		}
		return "", err
	}
	return target, nil
}

// Duplicate 在同目录复制 path 为副本（名字加 -2/-3 序号防覆盖）。返回副本路径。
func (s *Service) Duplicate(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", errEmptyPath
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	target := uniqueTarget(path)
	if err := copyPath(path, target, fi.IsDir()); err != nil {
		return "", err
	}
	return target, nil
}

// Trash 把 path 移到系统回收站（可恢复，非永久删除）。平台实现见 trash_<os>.go。
func (s *Service) Trash(ctx context.Context, path string) error {
	if path == "" {
		return errEmptyPath
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return trashPath(ctx, path)
}

// newChildPath 校验 name 并拼出 dir 下的目标路径（dir 必须是已存在目录）。
func newChildPath(dir, name string) (string, error) {
	if dir == "" {
		return "", errEmptyPath
	}
	if err := validName(name); err != nil {
		return "", err
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return "", errNotDir
	}
	return filepath.Join(dir, name), nil
}

// validName 校验文件/目录名：非空、≤255、不是 . 或 ..、不含路径分隔符或 NUL（照搬 fanbox validName）。
func validName(name string) error {
	switch {
	case name == "":
		return errEmptyName
	case len(name) > 255:
		return errNameTooLong
	case name == "." || name == "..":
		return errBadName
	case strings.ContainsAny(name, `/\`+"\x00"):
		return errBadName
	default:
		return nil
	}
}

// uniqueTarget 若 p 已存在，则在扩展名前追加 -2/-3… 直到不冲突。
func uniqueTarget(p string) string {
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 2; ; i++ {
		cand := base + "-" + strconv.Itoa(i) + ext
		if _, err := os.Stat(cand); errors.Is(err, os.ErrNotExist) {
			return cand
		}
	}
}

// copyPath 递归复制文件或目录 src 到 dst。
func copyPath(src, dst string, isDir bool) error {
	if !isDir {
		return copyFile(src, dst)
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

// copyFile 复制单个文件（覆盖目标）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// mimeForPath 按扩展名推断 MIME（图片优先用内置表，避免依赖系统注册表的不确定性）。
func mimeForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".avif":
		return "image/avif"
	}
	if m := mime.TypeByExtension(filepath.Ext(path)); m != "" {
		return m
	}
	return "application/octet-stream"
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
