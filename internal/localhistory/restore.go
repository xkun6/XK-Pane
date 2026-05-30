package localhistory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// putAutoLabel 构造并写入一个 auto 标签（uuid id + UTC RFC3339 时间戳）。供 CreateAutoLabel
// 与 restore 前的 "Before Restore/Rollback" 标签复用。
func (s *Store) putAutoLabel(ctx context.Context, name, source, branch string, snaps []LabelFileSnapshot) (string, error) {
	id := uuid.NewString()
	label := HistoryLabel{
		ID: id, Name: name, LabelType: "auto", Source: source,
		Timestamp: time.Now().UTC().Format(time.RFC3339), Branch: branch,
		FileSnapshots: snaps,
	}
	if err := s.PutLabel(ctx, label); err != nil {
		return "", err
	}
	return id, nil
}

// CreateAutoLabel 快照当前所有已跟踪文件的最新版本并打一个 auto 标签，返回标签 id。
// 对齐 cc-pane create_auto_label（git / claude / build / branch_switch 等钩子调用——其中
// gitx Pull/Push/Stash 的 auto-label 钩子由 gitsvc 服务层在调用 gitx 前调本方法）。
// branch 由调用方（服务层）经 gitx.CurrentBranch 计算后传入（""=未知 / 非 git 仓）。
func (s *Store) CreateAutoLabel(ctx context.Context, name, source, branch string) (string, error) {
	snaps, err := s.GetAllLatestSnapshots(ctx)
	if err != nil {
		return "", err
	}
	return s.putAutoLabel(ctx, name, source, branch, snaps)
}

// writeWorkingFile 把内容写回工作树文件（projectPath/relPath）。先建父目录（恢复已删文件需要）。
// 用 os.WriteFile 而非 filex.WriteAtomic：对**已存在文件** os.WriteFile 截断写入会保留原权限
// （可执行位等），而 temp+rename 的原子写会让文件变成新 inode、权限被重置——恢复工作树文件
// 须保权限，故这里不用原子写（blob 写仍用原子写，那是内部内容寻址文件，权限无关紧要）。
func (s *Store) writeWorkingFile(relPath string, content []byte) error {
	full := filepath.Join(s.projectPath, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, content, 0o644)
}

// RestoreVersion 把 filePath 恢复到 versionID 的内容（写回工作树）。恢复前自动打
// "Before Restore: <file>" 标签（仅当有可快照内容时），便于撤销本次恢复。对齐 cc-pane。
func (s *Store) RestoreVersion(ctx context.Context, filePath, versionID, branch string) error {
	snaps, err := s.GetAllLatestSnapshots(ctx)
	if err != nil {
		return err
	}
	if len(snaps) > 0 {
		if _, err := s.putAutoLabel(ctx, "Before Restore: "+filePath, "restore", branch, snaps); err != nil {
			return err
		}
	}
	content, err := s.GetVersionContent(ctx, filePath, versionID)
	if err != nil {
		return err
	}
	return s.writeWorkingFile(filePath, content)
}

// RestoreToLabel 把某标签捕获的所有文件快照恢复到工作树（Session 级回滚），返回成功恢复的
// 相对路径列表。恢复前自动打 "Before Rollback" 标签。标签不存在返回错误。逐文件 best-effort
// （取不到内容 / 写失败的快照跳过，不中断其余），对齐 cc-pane restore_to_label。
func (s *Store) RestoreToLabel(ctx context.Context, labelID, branch string) ([]string, error) {
	snaps, err := s.GetAllLatestSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	if len(snaps) > 0 {
		if _, err := s.putAutoLabel(ctx, "Before Rollback", "restore", branch, snaps); err != nil {
			return nil, err
		}
	}
	labels, err := s.ListLabels(ctx)
	if err != nil {
		return nil, err
	}
	var target *HistoryLabel
	for i := range labels {
		if labels[i].ID == labelID {
			target = &labels[i]
			break
		}
	}
	if target == nil {
		return nil, errors.New("localhistory: label not found")
	}
	restored := make([]string, 0, len(target.FileSnapshots))
	for _, snap := range target.FileSnapshots {
		content, err := s.GetVersionContent(ctx, snap.FilePath, snap.VersionID)
		if err != nil {
			continue
		}
		if err := s.writeWorkingFile(snap.FilePath, content); err != nil {
			continue
		}
		restored = append(restored, snap.FilePath)
	}
	return restored, nil
}
