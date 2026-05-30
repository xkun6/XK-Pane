package localhistory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cleanup 按项目配置清理：先按 age + 每文件数量删旧版本，再按总大小 LRU 删，最后回收孤儿 blob。
// 对齐 cc-pane history_service.cleanup。
func (s *Store) Cleanup(ctx context.Context) error {
	cfg, err := s.ReadConfig()
	if err != nil {
		return err
	}
	if err := s.CleanupOldVersions(ctx, cfg); err != nil {
		return err
	}
	return s.CleanupByTotalSize(ctx, cfg.MaxTotalSize)
}

// CleanupOldVersions 按 max_age_days（删早于 cutoff 的版本）+ max_versions_per_file（每文件只留
// 最新 N 个）清理，再回收孤儿 blob。对齐 cc-pane cleanup_old_versions。
func (s *Store) CleanupOldVersions(ctx context.Context, cfg HistoryConfig) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -cfg.MaxAgeDays).Format(time.RFC3339)

	var fps []string
	if err := s.gdb.WithContext(ctx).Model(&FileVersion{}).Distinct().Pluck("file_path", &fps).Error; err != nil {
		return err
	}
	for _, fp := range fps {
		// 1) 按时间删过期版本。
		if err := s.gdb.WithContext(ctx).
			Where("file_path = ? AND created_at < ?", fp, cutoff).
			Delete(&FileVersion{}).Error; err != nil {
			return err
		}
		// 2) 按数量限制：超出则删最旧的 excess 个（id ASC = 最旧）。
		var count int64
		if err := s.gdb.WithContext(ctx).Model(&FileVersion{}).
			Where("file_path = ?", fp).Count(&count).Error; err != nil {
			return err
		}
		if count > int64(cfg.MaxVersionsPerFile) {
			excess := int(count - int64(cfg.MaxVersionsPerFile))
			var oldIDs []string
			if err := s.gdb.WithContext(ctx).Model(&FileVersion{}).
				Where("file_path = ?", fp).Order("id ASC").Limit(excess).
				Pluck("id", &oldIDs).Error; err != nil {
				return err
			}
			if len(oldIDs) > 0 {
				if err := s.gdb.WithContext(ctx).
					Where("file_path = ? AND id IN ?", fp, oldIDs).
					Delete(&FileVersion{}).Error; err != nil {
					return err
				}
			}
		}
	}
	return s.cleanupOrphanBlobs(ctx)
}

// CleanupByTotalSize 当 blobs 目录总大小超过 maxTotalSize 时，按版本时间从旧到新删，直到释放足够
// （共享 blob 只在最后一个引用被删时计入释放量），再回收孤儿 blob。对齐 cc-pane cleanup_by_total_size。
func (s *Store) CleanupByTotalSize(ctx context.Context, maxTotalSize int64) error {
	total, err := s.blobsDirSize()
	if err != nil {
		return err
	}
	if total <= maxTotalSize {
		return nil
	}
	need := total - maxTotalSize

	var rows []FileVersion
	if err := s.gdb.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}

	var freed int64
	counted := make(map[string]struct{})
	for _, v := range rows {
		if freed >= need {
			break
		}
		if err := s.gdb.WithContext(ctx).
			Where("file_path = ? AND id = ?", v.FilePath, v.ID).
			Delete(&FileVersion{}).Error; err != nil {
			return err
		}
		// 共享 blob 不重复计释放量：仅首次遇到该 hash 时，若已无版本引用，计入其 blob 大小。
		if _, seen := counted[v.Hash]; !seen {
			counted[v.Hash] = struct{}{}
			var ref int64
			if err := s.gdb.WithContext(ctx).Model(&FileVersion{}).
				Where("hash = ?", v.Hash).Count(&ref).Error; err != nil {
				return err
			}
			if ref == 0 {
				if fi, err := os.Stat(s.blobPath(v.Hash)); err == nil {
					freed += fi.Size()
				}
			}
		}
	}
	return s.cleanupOrphanBlobs(ctx)
}

// cleanupOrphanBlobs 删除 blobs 目录中不再被任何版本引用的 blob 文件（跳过 .db* 防护文件）。
func (s *Store) cleanupOrphanBlobs(ctx context.Context) error {
	var hashes []string
	if err := s.gdb.WithContext(ctx).Model(&FileVersion{}).Distinct().Pluck("hash", &hashes).Error; err != nil {
		return err
	}
	referenced := make(map[string]struct{}, len(hashes))
	for _, h := range hashes {
		referenced[h] = struct{}{}
	}
	entries, err := os.ReadDir(s.blobs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".db-shm") {
			continue
		}
		if _, ok := referenced[name]; !ok {
			_ = os.Remove(filepath.Join(s.blobs, name))
		}
	}
	return nil
}

// HistoryStats 是项目历史存储的概要统计（供 UI 展示存储占用 + 清理前后对比，
// 对齐 cc-pane history get_storage_stats）。
type HistoryStats struct {
	VersionCount  int64 `json:"versionCount"`  // file_versions 行数（含删除标记版本）
	FileCount     int64 `json:"fileCount"`     // 去重文件数（distinct file_path）
	TotalBlobSize int64 `json:"totalBlobSize"` // blobs 目录总字节（不含 db / wal / shm）
}

// Stats 返回项目历史存储概要（版本数 / 文件数 / blob 总字节）。让清理操作可被观测。
func (s *Store) Stats(ctx context.Context) (HistoryStats, error) {
	var st HistoryStats
	if err := s.gdb.WithContext(ctx).Model(&FileVersion{}).Count(&st.VersionCount).Error; err != nil {
		return st, err
	}
	if err := s.gdb.WithContext(ctx).Model(&FileVersion{}).Distinct("file_path").Count(&st.FileCount).Error; err != nil {
		return st, err
	}
	size, err := s.blobsDirSize()
	if err != nil {
		return st, err
	}
	st.TotalBlobSize = size
	return st, nil
}

// blobsDirSize 计算 blobs 目录所有 blob 文件的总字节数。
func (s *Store) blobsDirSize() (int64, error) {
	entries, err := os.ReadDir(s.blobs)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".db-shm") {
			continue
		}
		if fi, err := e.Info(); err == nil {
			total += fi.Size()
		}
	}
	return total, nil
}
