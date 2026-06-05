// Package workspace 提供「Workspace 两层树」（workspaces + workspace_projects 两表）的
// CRUD / 排序 service —— P3a。
//
// 对照 cc-pane workspace_service.rs（文件存储 workspace.json）做 SQLite 化重建：
//   - 关系建模两张表（见 internal/storage/models_workspace.go 文件头偏离说明）。
//   - 排序：List = Pinned 优先 → SortOrder 升序(NULL 垫底) → CreatedAt 升序
//     （对照 workspace_service.rs:193-206）。
//   - 项目去重：同一 workspace 内按「归一化 path」去重，且 **加项目幂等**
//     （命中已存在直接返回那条，不报错 —— 与全局 project.Add 的"已存在报错"不同，
//     避免阻断 workspace 反复导入同一目录）。存储保留 **原始 path**（launchProject
//     要拿它当 PTY cwd，归一化仅用于比较）。
//   - 仅实现 P3a UI 真正消费的方法（含 @dnd-kit 拖拽排序的 Reorder/ReorderProjects）；
//     MoveProject(跨 ws 拖放) / SetProvider / SetHidden 暂无 UI，留作后续单元。
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"

	"xkpane/internal/storage"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 业务错误：纯英文字面串，风格对齐 project / provider service。
var (
	errNameEmpty        = errors.New("Workspace name cannot be empty")
	errNameExists       = errors.New("Workspace already exists")
	errWorkspaceMissing = errors.New("Workspace does not exist")
	errProjectPathEmpty = errors.New("Project path cannot be empty")
	errProjectMissing   = errors.New("Workspace project does not exist")
	errReorderMismatch  = errors.New("Reorder ids do not match existing rows")
	errSSHHostEmpty     = errors.New("SSH host cannot be empty")
)

// SshProjectConfig 是「SSH 远程项目」的连接信息（去规范化字段 + machineId 引用）。
// 落 WorkspaceProject.SSH 列（JSON）；启动时前端把它转成 terminal.SSHSpec。不含密码——
// 密码由 sshmachine.ResolveForSSH 据 MachineID 在后端解密，绝不入此 JSON。
type SshProjectConfig struct {
	MachineID    string `json:"machineId"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	RemotePath   string `json:"remotePath"`
	IdentityFile string `json:"identityFile"`
	AuthMethod   string `json:"authMethod"`
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

// List 返回全部工作空间（含内联 projects），按 Pinned → SortOrder(NULL 垫底) →
// CreatedAt 排序；每个工作空间内的 projects 再按 SortOrder → CreatedAt。
// 空结果返回非 nil 空切片，使 binding 序列化为 [] 而非 null。
func (s *Service) List(ctx context.Context) ([]storage.Workspace, error) {
	out := make([]storage.Workspace, 0)
	err := s.db().WithContext(ctx).
		Preload("Projects", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC").Order("created_at ASC")
		}).
		Order("pinned DESC").
		Order("sort_order IS NULL"). // 0(非空) 在前、1(NULL) 垫底
		Order("sort_order ASC").
		Order("created_at ASC").
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	// 保证每个 workspace 的 Projects 是非 nil 空切片（无项目时 → [] 而非 null）。
	for i := range out {
		if out[i].Projects == nil {
			out[i].Projects = []storage.WorkspaceProject{}
		}
	}
	return out, nil
}

// Create 新建工作空间。name 去空白后判空 + 唯一校验；path 去空白后空串 → NULL。
func (s *Service) Create(ctx context.Context, name string, path *string) (storage.Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return storage.Workspace{}, errNameEmpty
	}
	exists, err := s.nameExists(ctx, name, "")
	if err != nil {
		return storage.Workspace{}, err
	}
	if exists {
		return storage.Workspace{}, errNameExists
	}
	ws := storage.Workspace{
		ID:                 uuid.NewString(),
		Name:               name,
		Path:               cleanOptStr(path),
		DefaultEnvironment: "local",
	}
	if err := s.db().WithContext(ctx).Create(&ws).Error; err != nil {
		if isUniqueViolation(err) { // 预检与入库之间的竞态兜底
			return storage.Workspace{}, errNameExists
		}
		return storage.Workspace{}, err
	}
	ws.Projects = []storage.WorkspaceProject{}
	return ws, nil
}

// Rename 改工作空间显示名（仅改 name 列，无磁盘目录 rename）。去空白判空 + 唯一校验。
func (s *Service) Rename(ctx context.Context, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errNameEmpty
	}
	exists, err := s.nameExists(ctx, name, id)
	if err != nil {
		return err
	}
	if exists {
		return errNameExists
	}
	res := s.db().WithContext(ctx).Model(&storage.Workspace{}).
		Where("id = ?", id).Update("name", name)
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return errNameExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errWorkspaceMissing
	}
	return nil
}

// Delete 删除工作空间，事务内级联删其下 workspace_projects。
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", id).Delete(&storage.WorkspaceProject{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&storage.Workspace{}, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errWorkspaceMissing
		}
		return nil
	})
}

// SetAlias 设/清工作空间别名（nil/空/纯空白 → NULL）。
func (s *Service) SetAlias(ctx context.Context, id string, alias *string) error {
	return s.updateWorkspaceCol(ctx, id, "alias", cleanOptStr(alias))
}

// SetPath 设/清工作空间根目录路径（nil/空 → NULL）。
func (s *Service) SetPath(ctx context.Context, id string, path *string) error {
	return s.updateWorkspaceCol(ctx, id, "path", cleanOptStr(path))
}

// SetPinned 置顶 / 取消置顶。
func (s *Service) SetPinned(ctx context.Context, id string, pinned bool) error {
	return s.updateWorkspaceCol(ctx, id, "pinned", pinned)
}

// AddProject 在工作空间下登记一个项目节点。归一化 path 去重 + 幂等（命中已存在直接返回）。
// SortOrder 追加到末尾。存储保留原始 path。
func (s *Service) AddProject(ctx context.Context, workspaceID, path string) (storage.WorkspaceProject, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return storage.WorkspaceProject{}, errProjectPathEmpty
	}
	wsExists, err := s.workspaceExists(ctx, workspaceID)
	if err != nil {
		return storage.WorkspaceProject{}, err
	}
	if !wsExists {
		return storage.WorkspaceProject{}, errWorkspaceMissing
	}

	var siblings []storage.WorkspaceProject
	if err := s.db().WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("sort_order ASC").Find(&siblings).Error; err != nil {
		return storage.WorkspaceProject{}, err
	}
	norm := normalizeProjectPath(path)
	for _, sib := range siblings {
		if normalizeProjectPath(sib.Path) == norm {
			return sib, nil // 幂等：已存在则直接返回
		}
	}

	wp := storage.WorkspaceProject{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Path:        path, // 原样存储
		SortOrder:   len(siblings),
	}
	if err := s.db().WithContext(ctx).Create(&wp).Error; err != nil {
		if isUniqueViolation(err) {
			// 竞态：并发插入同一原始 path —— 回查返回已存在记录，保持幂等。
			var existing storage.WorkspaceProject
			if e := s.db().WithContext(ctx).
				Where("workspace_id = ? AND path = ?", workspaceID, path).
				First(&existing).Error; e == nil {
				return existing, nil
			}
		}
		return storage.WorkspaceProject{}, err
	}
	return wp, nil
}

// AddSSHProject 在工作空间下登记一个「SSH 远程项目」节点。远端无本地 path，故用合成键
// `ssh://<machineId 或 host>/<remotePath>` 填 Path 满足 (workspaceID, path) 唯一约束；
// cfg 序列化进 SSH 列。按合成键去重 + 幂等（命中已存在直接返回）。SortOrder 追加末尾。
func (s *Service) AddSSHProject(ctx context.Context, workspaceID string, cfg SshProjectConfig) (storage.WorkspaceProject, error) {
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		return storage.WorkspaceProject{}, errSSHHostEmpty
	}
	if cfg.Port <= 0 {
		cfg.Port = 22
	}
	cfg.AuthMethod = strings.TrimSpace(cfg.AuthMethod)
	cfg.RemotePath = strings.TrimSpace(cfg.RemotePath)

	wsExists, err := s.workspaceExists(ctx, workspaceID)
	if err != nil {
		return storage.WorkspaceProject{}, err
	}
	if !wsExists {
		return storage.WorkspaceProject{}, errWorkspaceMissing
	}

	syntheticPath := sshSyntheticPath(cfg)

	var siblings []storage.WorkspaceProject
	if err := s.db().WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("sort_order ASC").Find(&siblings).Error; err != nil {
		return storage.WorkspaceProject{}, err
	}
	norm := normalizeProjectPath(syntheticPath)
	for _, sib := range siblings {
		if normalizeProjectPath(sib.Path) == norm {
			return sib, nil // 幂等：同机器+同远端路径已存在则直接返回
		}
	}

	blob, err := json.Marshal(cfg)
	if err != nil {
		return storage.WorkspaceProject{}, err
	}
	js := string(blob)
	wp := storage.WorkspaceProject{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Path:        syntheticPath,
		SSH:         &js,
		SortOrder:   len(siblings),
	}
	if err := s.db().WithContext(ctx).Create(&wp).Error; err != nil {
		if isUniqueViolation(err) {
			var existing storage.WorkspaceProject
			if e := s.db().WithContext(ctx).
				Where("workspace_id = ? AND path = ?", workspaceID, syntheticPath).
				First(&existing).Error; e == nil {
				return existing, nil
			}
		}
		return storage.WorkspaceProject{}, err
	}
	return wp, nil
}

// sshSyntheticPath 为 SSH 远程项目生成稳定唯一的 Path 键：`ssh://<machineId 或 host>/<remotePath>`。
// machineId 优先（稳定，机器改 host 不破键）；缺省 host。remotePath 空 → "~"。
func sshSyntheticPath(cfg SshProjectConfig) string {
	key := cfg.MachineID
	if key == "" {
		key = cfg.Host
	}
	rp := cfg.RemotePath
	if rp == "" {
		rp = "~"
	}
	return "ssh://" + key + "/" + strings.TrimPrefix(rp, "/")
}

// RemoveProject 按 id 删除项目节点。
func (s *Service) RemoveProject(ctx context.Context, projectID string) error {
	res := s.db().WithContext(ctx).Delete(&storage.WorkspaceProject{}, "id = ?", projectID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errProjectMissing
	}
	return nil
}

// SetProjectAlias 设/清项目节点别名（nil/空/纯空白 → NULL）。
func (s *Service) SetProjectAlias(ctx context.Context, projectID string, alias *string) error {
	res := s.db().WithContext(ctx).Model(&storage.WorkspaceProject{}).
		Where("id = ?", projectID).Update("alias", cleanOptStr(alias))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errProjectMissing
	}
	return nil
}

// Reorder 按给定顺序重写工作空间 sort_order（@dnd-kit 拖拽消费）。
// 校验：ids 无重复、全部存在；逐个写 sort_order = 位置下标。前端传"该组的完整有序列表"。
func (s *Service) Reorder(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if hasDup(ids) {
		return errReorderMismatch
	}
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&storage.Workspace{}).Where("id IN ?", ids).Count(&n).Error; err != nil {
			return err
		}
		if int(n) != len(ids) {
			return errReorderMismatch
		}
		for i, id := range ids {
			if err := tx.Model(&storage.Workspace{}).Where("id = ?", id).
				Update("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ReorderProjects 按给定顺序重写某工作空间下项目节点的 sort_order（@dnd-kit 拖拽消费）。
func (s *Service) ReorderProjects(ctx context.Context, workspaceID string, projectIDs []string) error {
	if len(projectIDs) == 0 {
		return nil
	}
	if hasDup(projectIDs) {
		return errReorderMismatch
	}
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&storage.WorkspaceProject{}).
			Where("workspace_id = ? AND id IN ?", workspaceID, projectIDs).
			Count(&n).Error; err != nil {
			return err
		}
		if int(n) != len(projectIDs) {
			return errReorderMismatch
		}
		for i, id := range projectIDs {
			if err := tx.Model(&storage.WorkspaceProject{}).
				Where("id = ? AND workspace_id = ?", id, workspaceID).
				Update("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// —— 内部 helper ——

func (s *Service) updateWorkspaceCol(ctx context.Context, id, col string, val any) error {
	res := s.db().WithContext(ctx).Model(&storage.Workspace{}).
		Where("id = ?", id).Update(col, val)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errWorkspaceMissing
	}
	return nil
}

func (s *Service) nameExists(ctx context.Context, name, excludeID string) (bool, error) {
	q := s.db().WithContext(ctx).Model(&storage.Workspace{}).Where("name = ?", name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	err := q.Count(&n).Error
	return n > 0, err
}

func (s *Service) workspaceExists(ctx context.Context, id string) (bool, error) {
	var n int64
	err := s.db().WithContext(ctx).Model(&storage.Workspace{}).Where("id = ?", id).Count(&n).Error
	return n > 0, err
}

// cleanOptStr：nil / 空串 / 纯空白 → nil（SQL NULL）；否则 trim 后的值。
func cleanOptStr(p *string) *string {
	if p == nil {
		return nil
	}
	if t := strings.TrimSpace(*p); t != "" {
		return &t
	}
	return nil
}

// normalizeProjectPath：仅用于 workspace 内项目去重比较（不改存储值）。
// 统一斜杠 + 去尾斜杠；Windows 路径大小写不敏感 → 转小写（对照 cc-pane
// normalize_project_path workspace_service.rs:312-319）。
func normalizeProjectPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimRight(p, "/")
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

func hasDup(ids []string) bool {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}

// isUniqueViolation 识别 modernc/sqlite 的 UNIQUE 约束冲突（GORM 未开 TranslateError，
// 只能匹配驱动错误串）。与 project service 同款兜底。
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
