// Package specsvc 管理「需求规格（Spec）」文档（specs 表 + 每项目 .xkpanes/specs/ 下的 .md 文件）。
//
// 移植 cc-pane spec_repo + spec_service 的**元数据 CRUD + 文件生命周期**子集：
//   - Create：插 DB 元数据 + 从模板生成 <project>/.xkpanes/specs/<slug>.spec.md（失败补偿删 DB 行）
//   - List / Get / GetActive：浏览（按项目、可选状态过滤）
//   - GetContent / SaveContent：读 / 写 .md 文件内容（SaveContent 顺带 bump updated_at）
//   - Update：改标题 / 状态；状态机——active 先把同项目其它 active 降级 draft，archived 移文件到
//     archived/ 子目录并记 archived_at
//   - Delete：删文件 + DB 行
//
// 偏离 cc-pane：目录 `.ccpanes/specs` → `.xkpanes/specs`（对齐 D10 每项目 .xkpanes）。
// **延后**（独立的跨服务 / 高级单元）：关联 Todo（todoType="spec"）、Tasks 段 checkbox 双向同步、
// Log 段追加、active spec 摘要注入终端。故本切片 Create **不**建关联 Todo、Delete **不**级联 Todo。
//
// 存储：specs 表在 app 级单库（AllModels，走 Holder），按 project_path 列查（同 todo/session）。
package specsvc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"xkpane/internal/storage"
	"xkpane/internal/utils/filex"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	xkpanesDir  = ".xkpanes"
	specsSubdir = "specs"
	archivedSub = "archived"
)

// 合法状态集（对齐 cc-pane SpecStatus）。
var validStatus = map[string]bool{"draft": true, "active": true, "archived": true}

type Service struct {
	holder *storage.Holder
}

func New(holder *storage.Holder) *Service { return &Service{holder: holder} }

func (s *Service) db() *gorm.DB { return s.holder.Current().GORM }

func specDir(projectPath string) string {
	return filepath.Join(projectPath, xkpanesDir, specsSubdir)
}

// resolveSpecPath 给出某 spec 的 .md 绝对路径；archived 状态在 archived/ 子目录。
func resolveSpecPath(projectPath string, entry storage.Spec) string {
	if entry.Status == "archived" {
		return filepath.Join(specDir(projectPath), archivedSub, entry.FileName)
	}
	return filepath.Join(specDir(projectPath), entry.FileName)
}

// titleToFilename 把标题转成 `<slug>.spec.md`：小写、非字母数字→'-'、折叠连续'-'、去首尾'-'。
// 字母 / 数字判定用 Unicode 感知（保留 CJK，对齐 cc-pane is_alphanumeric 行为）。
func titleToFilename(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	parts := strings.FieldsFunc(b.String(), func(r rune) bool { return r == '-' })
	return strings.Join(parts, "-") + ".spec.md"
}

// renderTemplate 渲染初始 Spec 文档（对齐 cc-pane render_template；date 取 createdAt 前 10 位）。
func renderTemplate(title, date string) string {
	return fmt.Sprintf(`# Spec: %s
> Status: draft | Created: %s

## Proposal
变更动机和目标。

## Design
技术方案。

### Affected Files
- `+"`src/xxx.ts`"+` [ADDED]
- `+"`src/yyy.ts`"+` [MODIFIED]

## Tasks (auto-synced from CC-Panes)
<!-- 此段由 CC-Panes 自动同步，请在 Todo 面板中编辑 -->

## Log
`, title, date)
}

// Create 新建 Spec：DB 插元数据（status=draft）+ 从模板写 .md 文件。文件失败则补偿删 DB 行。
func (s *Service) Create(ctx context.Context, projectPath, title string) (storage.Spec, error) {
	t := strings.TrimSpace(title)
	if t == "" {
		return storage.Spec{}, errors.New("specsvc: spec title cannot be empty")
	}
	if strings.TrimSpace(projectPath) == "" {
		return storage.Spec{}, errors.New("specsvc: project path required")
	}
	now := time.Now().UTC()
	entry := storage.Spec{
		ID:          uuid.NewString(),
		ProjectPath: projectPath,
		Title:       t,
		FileName:    titleToFilename(t),
		Status:      "draft",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db().WithContext(ctx).Create(&entry).Error; err != nil {
		return storage.Spec{}, err
	}
	// 写模板文件（WriteAtomic 自建父目录）。失败补偿：删 DB 行。
	path := filepath.Join(specDir(projectPath), entry.FileName)
	content := renderTemplate(t, now.Format(time.RFC3339)[:10])
	if err := filex.WriteAtomic(path, []byte(content), 0o644); err != nil {
		_ = s.db().WithContext(ctx).Where("id = ?", entry.ID).Delete(&storage.Spec{})
		return storage.Spec{}, fmt.Errorf("specsvc: write spec file: %w", err)
	}
	return entry, nil
}

// List 列出某项目的 Spec（status 非空则过滤），按 created_at 倒序。
func (s *Service) List(ctx context.Context, projectPath, status string) ([]storage.Spec, error) {
	q := s.db().WithContext(ctx).Where("project_path = ?", projectPath)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	rows := make([]storage.Spec, 0)
	err := q.Order("created_at DESC").Find(&rows).Error
	return rows, err
}

// Get 按 id 查；不存在返回 (nil, nil)。
func (s *Service) Get(ctx context.Context, id string) (*storage.Spec, error) {
	var sp storage.Spec
	err := s.db().WithContext(ctx).First(&sp, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

// GetActive 返回某项目的 active spec；无则 (nil, nil)。
func (s *Service) GetActive(ctx context.Context, projectPath string) (*storage.Spec, error) {
	var sp storage.Spec
	err := s.db().WithContext(ctx).Where("project_path = ? AND status = ?", projectPath, "active").
		Limit(1).First(&sp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

// GetContent 读取某 spec 的 .md 文件内容（限长读，默认 8MB 上限）。
func (s *Service) GetContent(ctx context.Context, projectPath, id string) (string, error) {
	entry, err := s.getOr404(ctx, id)
	if err != nil {
		return "", err
	}
	data, err := filex.ReadLimit(resolveSpecPath(projectPath, *entry), 0)
	if err != nil {
		return "", fmt.Errorf("specsvc: read spec file: %w", err)
	}
	return string(data), nil
}

// SaveContent 原子写某 spec 的 .md 文件，并 bump updated_at。
func (s *Service) SaveContent(ctx context.Context, projectPath, id, content string) error {
	entry, err := s.getOr404(ctx, id)
	if err != nil {
		return err
	}
	if err := filex.WriteAtomic(resolveSpecPath(projectPath, *entry), []byte(content), 0o644); err != nil {
		return fmt.Errorf("specsvc: write spec file: %w", err)
	}
	return s.db().WithContext(ctx).Model(&storage.Spec{}).Where("id = ?", id).
		Update("updated_at", time.Now().UTC()).Error
}

// Update 改 Spec 元数据（title / status 可选）。状态机：
//   - active：先把同项目其它 active 降级为 draft（保证至多一个 active，对齐 cc-pane）；
//   - archived：把 .md 移到 archived/ 子目录（带日期前缀防冲突）并记 archived_at；
//   - draft：允许从 active 回退，无副作用。
func (s *Service) Update(ctx context.Context, id string, title *string, status *string) (*storage.Spec, error) {
	entry, err := s.getOr404(ctx, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if title != nil {
		tt := strings.TrimSpace(*title)
		if tt == "" {
			return nil, errors.New("specsvc: spec title cannot be empty")
		}
		updates["title"] = tt
	}
	if status != nil {
		if !validStatus[*status] {
			return nil, fmt.Errorf("specsvc: invalid status %q", *status)
		}
		switch *status {
		case "active":
			if err := s.deactivateAll(ctx, entry.ProjectPath); err != nil {
				return nil, err
			}
		case "archived":
			if err := s.archiveSpecFile(ctx, entry); err != nil {
				return nil, err
			}
			updates["archived_at"] = time.Now().UTC()
		}
		updates["status"] = *status
	}
	if len(updates) > 0 {
		if err := s.db().WithContext(ctx).Model(&storage.Spec{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, id)
}

// Delete 删除某 spec：先删 .md 文件（best-effort），再删 DB 行。无匹配行报错。
func (s *Service) Delete(ctx context.Context, projectPath, id string) error {
	entry, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return errors.New("specsvc: spec not found")
	}
	if path := resolveSpecPath(projectPath, *entry); path != "" {
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			// 文件删除失败不阻断 DB 清理（best-effort，对齐 cc-pane）
			_ = rmErr
		}
	}
	return s.db().WithContext(ctx).Where("id = ?", id).Delete(&storage.Spec{}).Error
}

// deactivateAll 把某项目所有 active spec 降级为 draft（updated_at 由 GORM 自动 bump）。
func (s *Service) deactivateAll(ctx context.Context, projectPath string) error {
	return s.db().WithContext(ctx).Model(&storage.Spec{}).
		Where("project_path = ? AND status = ?", projectPath, "active").
		Update("status", "draft").Error
}

// archiveSpecFile 把 entry 的 .md 从 specs/ 移到 specs/archived/<date>_<file>，并把 DB 的 file_name
// 更新为归档后的文件名（与 resolveSpecPath 的 archived 分支一致）。源文件不存在则静默跳过。
func (s *Service) archiveSpecFile(ctx context.Context, entry *storage.Spec) error {
	dir := specDir(entry.ProjectPath)
	archivedDir := filepath.Join(dir, archivedSub)
	if err := os.MkdirAll(archivedDir, 0o755); err != nil {
		return fmt.Errorf("specsvc: create archived dir: %w", err)
	}
	src := filepath.Join(dir, entry.FileName)
	if _, statErr := os.Stat(src); statErr != nil {
		return nil // 源文件不存在，无需移动
	}
	archivedName := time.Now().Format("20060102") + "_" + entry.FileName
	if err := os.Rename(src, filepath.Join(archivedDir, archivedName)); err != nil {
		return fmt.Errorf("specsvc: archive spec file: %w", err)
	}
	// 更新 file_name，使 resolveSpecPath（archived 分支）指向归档后的文件。
	return s.db().WithContext(ctx).Model(&storage.Spec{}).Where("id = ?", entry.ID).
		Update("file_name", archivedName).Error
}

func (s *Service) getOr404(ctx context.Context, id string) (*storage.Spec, error) {
	entry, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("specsvc: spec %s not found", id)
	}
	return entry, nil
}
