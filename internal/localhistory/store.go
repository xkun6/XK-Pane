// Package localhistory 是「本地历史」（文件版本快照）的每项目存储基座。
//
// 架构（决策门 D10，用户定）：**每个项目一个独立存储**，位于 <project>/.xkpanes/history/：
//   - history.db：SQLite（GORM/glebarez + AutoMigrate），file_versions 元数据表
//   - blobs/：内容寻址 blob 文件（文件名 = SHA256(content)，跨版本去重）
//
// 这是相对 foundation D8（app 级状态走单一 xkpane.db + Holder + AllModels）的**有意分层**：
// 文件版本历史是「每项目、随项目走、内容可能很大」的独立存储域，不进 app 主库、不走 Holder，
// 改用「按项目打开一个 *gorm.DB」的多 db 模式（服务层后续按项目缓存 Store）。目录名 .xkpanes
// （对齐 XK-Pane 品牌；cc-pane 原版是 .ccpanes）。移植自 cc-pane history_file_repo.rs。
//
// 本单元 = 存储基座：Open/Close + SaveVersion（hash 去重 + 间隔节流 + blob 落盘）+
// ListVersions(_ByBranch) + GetFileBranches + GetVersionContent（gzip 兼容读）。
// 后续单元：diff 引擎 / 标签 / 回滚 restore / 最近变更 / 清理 pruning（max_age/max_versions/
// max_total_size + 孤儿 blob 回收）/ gzip 压缩 / Wails 服务层 + auto-label 钩子（供 gitx
// Pull/Push/Stash 外层调用）。
package localhistory

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"xkpane/internal/utils/filex"
)

const (
	dirName    = ".xkpanes"
	historyDir = "history"
	blobsDir   = "blobs"
	dbFile     = "history.db"
)

// FileVersion 是一条文件版本元数据（内容存 blobs/<hash>）。复合主键 (FilePath, ID)。
// 对齐 cc-pane FileVersion。
type FileVersion struct {
	ID        string `gorm:"primaryKey;column:id" json:"id"`                    // 时间戳ms + 4位计数（字典序==时序）
	FilePath  string `gorm:"primaryKey;column:file_path;index" json:"filePath"` // 项目内相对路径
	Hash      string `gorm:"column:hash;index" json:"hash"`                     // SHA256(content) 十六进制
	Size      int64  `gorm:"column:size" json:"size"`
	CreatedAt string `gorm:"column:created_at;index" json:"createdAt"` // RFC3339 UTC
	IsDeleted bool   `gorm:"column:is_deleted" json:"isDeleted"`
	Branch    string `gorm:"column:branch;index" json:"branch"` // 所属 git 分支（空 = 未知）
}

// TableName 固定表名，避免 GORM 复数化推断漂移。
func (FileVersion) TableName() string { return "file_versions" }

// Store 是一个已打开的项目历史存储。桌面单用户 + WAL，单连接足够。
type Store struct {
	gdb         *gorm.DB
	projectPath string
	blobs       string
}

// versionCounter 给同毫秒内的多次保存补 4 位序号，避免主键 (file_path,id) 撞。
var versionCounter uint32

// Open 打开 / 创建 projectPath 的历史存储（建 .xkpanes/history/blobs + history.db + AutoMigrate）。
func Open(projectPath string) (*Store, error) {
	if projectPath == "" {
		return nil, errors.New("localhistory: empty project path")
	}
	base := filepath.Join(projectPath, dirName, historyDir)
	blobs := filepath.Join(base, blobsDir)
	if err := os.MkdirAll(blobs, 0o755); err != nil {
		return nil, fmt.Errorf("localhistory: mkdir blobs: %w", err)
	}
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(base, dbFile)), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("localhistory: open db: %w", err)
	}
	for _, p := range []string{
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA synchronous = NORMAL;`,
		`PRAGMA busy_timeout = 5000;`,
	} {
		if err := gdb.Exec(p).Error; err != nil {
			return nil, fmt.Errorf("localhistory: pragma: %w", err)
		}
	}
	if err := gdb.AutoMigrate(&FileVersion{}); err != nil {
		return nil, fmt.Errorf("localhistory: migrate: %w", err)
	}
	return &Store{gdb: gdb, projectPath: projectPath, blobs: blobs}, nil
}

// Close 关闭底层连接池。
func (s *Store) Close() error {
	if s == nil || s.gdb == nil {
		return nil
	}
	sqlDB, err := s.gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func (s *Store) blobPath(hash string) string { return filepath.Join(s.blobs, hash) }

// SaveVersion 保存 filePath 的一个版本。返回保存的版本；若因去重 / 节流跳过则返回 (nil, nil)。
//   - 去重：同 filePath+branch 的最新版 hash 相同 → 跳过（is_deleted 版本不去重）。
//   - 节流：minIntervalSecs>0 且距最新版 < 该间隔 → 跳过（is_deleted 版本不受节流约束）。
//   - blob 内容寻址落盘（同 hash 不重复写），原子写。
func (s *Store) SaveVersion(ctx context.Context, filePath string, content []byte, isDeleted bool, branch string, minIntervalSecs int64) (*FileVersion, error) {
	hash := hashContent(content)

	if !isDeleted {
		var latest FileVersion
		if err := s.gdb.WithContext(ctx).
			Where("file_path = ? AND branch = ?", filePath, branch).
			Order("id DESC").Limit(1).Find(&latest).Error; err != nil {
			return nil, err
		}
		if latest.ID != "" {
			if latest.Hash == hash {
				return nil, nil // 内容未变 → 去重跳过
			}
			if minIntervalSecs > 0 {
				if last, perr := time.Parse(time.RFC3339, latest.CreatedAt); perr == nil {
					if time.Since(last) < time.Duration(minIntervalSecs)*time.Second {
						return nil, nil // 间隔不足 → 节流跳过
					}
				}
			}
		}
	}

	// 写 blob（内容寻址去重：仅当该 hash 文件不存在时才写；原子写防崩溃半包）。
	bp := s.blobPath(hash)
	if _, err := os.Stat(bp); errors.Is(err, os.ErrNotExist) {
		if err := filex.WriteAtomic(bp, content, 0o644); err != nil {
			return nil, fmt.Errorf("localhistory: write blob: %w", err)
		}
	}

	now := time.Now().UTC()
	counter := atomic.AddUint32(&versionCounter, 1) % 10000
	v := FileVersion{
		ID:        fmt.Sprintf("%d%04d", now.UnixMilli(), counter),
		FilePath:  filePath,
		Hash:      hash,
		Size:      int64(len(content)),
		CreatedAt: now.Format(time.RFC3339),
		IsDeleted: isDeleted,
		Branch:    branch,
	}
	if err := s.gdb.WithContext(ctx).Create(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// ListVersions 列出 filePath 的所有版本（按 id 升序，即时序）。
func (s *Store) ListVersions(ctx context.Context, filePath string) ([]FileVersion, error) {
	out := make([]FileVersion, 0)
	err := s.gdb.WithContext(ctx).Where("file_path = ?", filePath).Order("id ASC").Find(&out).Error
	return out, err
}

// ListVersionsByBranch 列出 filePath 在指定 branch 的版本（id 升序）。
func (s *Store) ListVersionsByBranch(ctx context.Context, filePath, branch string) ([]FileVersion, error) {
	out := make([]FileVersion, 0)
	err := s.gdb.WithContext(ctx).Where("file_path = ? AND branch = ?", filePath, branch).Order("id ASC").Find(&out).Error
	return out, err
}

// GetFileBranches 返回 filePath 有版本记录的所有分支（去重，升序）。
func (s *Store) GetFileBranches(ctx context.Context, filePath string) ([]string, error) {
	out := make([]string, 0)
	err := s.gdb.WithContext(ctx).Model(&FileVersion{}).
		Where("file_path = ?", filePath).Distinct().Order("branch").Pluck("branch", &out).Error
	return out, err
}

// GetVersionContent 取某版本的内容（按 hash 读 blob；gzip 魔数则解压，前向兼容未来压缩单元）。
// 版本不存在返回错误。
func (s *Store) GetVersionContent(ctx context.Context, filePath, versionID string) ([]byte, error) {
	var v FileVersion
	if err := s.gdb.WithContext(ctx).
		Where("file_path = ? AND id = ?", filePath, versionID).Limit(1).Find(&v).Error; err != nil {
		return nil, err
	}
	if v.ID == "" {
		return nil, errors.New("localhistory: version not found")
	}
	raw, err := os.ReadFile(s.blobPath(v.Hash))
	if err != nil {
		return nil, fmt.Errorf("localhistory: read blob: %w", err)
	}
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b { // gzip 魔数
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("localhistory: gzip: %w", err)
		}
		defer gr.Close()
		return io.ReadAll(gr)
	}
	return raw, nil
}
