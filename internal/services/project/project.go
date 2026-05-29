// Package project 提供"项目登记表"（projects 表）的 CRUD service。
//
// 忠实移植 cc-pane 的独立 Project 实体（ProjectService + ProjectRepository +
// project_commands）。cc-pane 另有一套"工作区内嵌项目"（WorkspaceProject，
// 基于 <data_dir>/workspaces/<name>/workspace.json 文件存储）——那是独立子系统
// （文件存储，非 SQLite），本服务不涉及，留作后续单元。
//
// 移植保真铁律（详见各方法注释与 findings）：
//   - Path = PTY cwd / 唯一操作目标（D7），原样存储，绝不规范化；去重为逐字节
//     字符串相等（C:\proj、C:/proj、C:\proj\ 视为三个不同项目）。
//   - Add 不是 upsert：重复路径报 "Project already exists"（service 预检 +
//     DB uniqueIndex 双层兜底，错误串一致）。
//   - List 按 created_at 倒序（最新在前）；空结果回非 nil 空切片（JSON []）。
//   - alias：trim 后空/纯空白/nil 一律清成 NULL；否则存 trim 后的值。
//   - name：创建时取路径末段（deriveName）；UpdateName 存"未 trim"原值
//     （仅用 trim 判空），与 alias 存 trim 值的非对称行为有意保留。
//   - id 是服务端生成的 UUIDv4；get/remove/update_* 一律按 id（绝不按 path）。
package project

import (
	"context"
	"errors"
	"os"
	"strings"

	"xkpane/internal/storage"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// service 层业务错误：纯英文字面串（code=None），与 cc-pane 逐字一致。
// （结构化、可 i18n 的错误码仅用于 validatePath，见 validate.go。）
var (
	errPathNotExist   = errors.New("Path does not exist")
	errPathNotDir     = errors.New("Path is not a directory")
	errProjectExists  = errors.New("Project already exists")
	errProjectMissing = errors.New("Project does not exist")
	errNameEmpty      = errors.New("Project name cannot be empty")
)

type Service struct {
	holder *storage.Holder
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// List 返回全部项目，按创建时间倒序（最新在前）。空结果返回非 nil 空切片，
// 使 Wails binding 序列化为 [] 而非 null。
func (s *Service) List(ctx context.Context) ([]storage.Project, error) {
	out := make([]storage.Project, 0)
	if err := s.db().WithContext(ctx).Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Add 登记一个新项目目录（非 upsert）。流程 fail-fast：校验路径 → 确认存在且为
// 目录 → 逐字节去重 → 生成记录（uuid id / 末段 name / 原样 path / alias=NULL）
// → 入库。返回创建出的记录。
func (s *Service) Add(ctx context.Context, path string) (storage.Project, error) {
	if err := validatePath(path); err != nil {
		return storage.Project{}, err
	}
	// PathBuf::exists() 对任何 stat 错误都视为 false → 统一映射"不存在"。
	fi, err := os.Stat(path)
	if err != nil {
		return storage.Project{}, errPathNotExist
	}
	if !fi.IsDir() {
		return storage.Project{}, errPathNotDir
	}
	exists, err := s.existsByPath(ctx, path)
	if err != nil {
		return storage.Project{}, err
	}
	if exists {
		return storage.Project{}, errProjectExists
	}
	p := storage.Project{
		ID:   uuid.NewString(),
		Name: deriveName(path),
		Path: path, // 原样存储（铁律：绝不规范化）
		// Alias 留 nil → NULL；CreatedAt/UpdatedAt 由 GORM 自动填。
	}
	if err := s.db().WithContext(ctx).Create(&p).Error; err != nil {
		// 预检与入库之间的竞态：DB uniqueIndex 冲突重映射成同一业务错误。
		if isUniqueViolation(err) {
			return storage.Project{}, errProjectExists
		}
		return storage.Project{}, err
	}
	return p, nil
}

// Get 按 id 查一个项目。不存在不是错误——返回 (nil, nil)，前端 binding 得到
// null（对齐 cc-pane AppResult<Option<Project>> 的 Ok(None)）。
func (s *Service) Get(ctx context.Context, id string) (*storage.Project, error) {
	var p storage.Project
	err := s.db().WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Remove 按 id 删除。无匹配行视为错误（"Project does not exist"）。仅按 id。
// 不做级联（launch_history / terminal_sessions / specs 不在本层 FK 级联，
// 与 cc-pane 一致）。
func (s *Service) Remove(ctx context.Context, id string) error {
	res := s.db().WithContext(ctx).Delete(&storage.Project{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errProjectMissing
	}
	return nil
}

// UpdateName 改名。空名（trim 后）报错且不触库；存储的是"未 trim"的原值
// （trim 仅用于判空——与 UpdateAlias 存 trim 值的行为有意非对称）。无匹配行
// 报 "Project does not exist"。
func (s *Service) UpdateName(ctx context.Context, id, name string) error {
	if strings.TrimSpace(name) == "" {
		return errNameEmpty
	}
	res := s.db().WithContext(ctx).Model(&storage.Project{}).
		Where("id = ?", id).Update("name", name)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errProjectMissing
	}
	return nil
}

// UpdateAlias 设/清别名。alias 为 nil、空串或纯空白 → 清成 NULL；否则存 trim
// 后的非空值。无匹配行报 "Project does not exist"。
func (s *Service) UpdateAlias(ctx context.Context, id string, alias *string) error {
	var v *string // nil → SQL NULL（清除）
	if alias != nil {
		if trimmed := strings.TrimSpace(*alias); trimmed != "" {
			v = &trimmed
		}
	}
	res := s.db().WithContext(ctx).Model(&storage.Project{}).
		Where("id = ?", id).Update("alias", v)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errProjectMissing
	}
	return nil
}

// existsByPath 逐字节路径相等去重原语（Add 用），不规范化。非导出 → 不进前端
// bindings（对齐 cc-pane：exists_by_path 仅 repository 内部、非 tauri 命令）。
func (s *Service) existsByPath(ctx context.Context, path string) (bool, error) {
	var n int64
	err := s.db().WithContext(ctx).Model(&storage.Project{}).
		Where("path = ?", path).Count(&n).Error
	return n > 0, err
}

// isUniqueViolation 识别 modernc/sqlite 的 UNIQUE 约束冲突。XK-Pane 的 GORM 未开
// TranslateError（见 storage/db.go），拿不到 gorm.ErrDuplicatedKey，只能匹配
// 驱动错误串。仅作 Add 预检之后的竞态兜底（正常去重走 existsByPath）。
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
