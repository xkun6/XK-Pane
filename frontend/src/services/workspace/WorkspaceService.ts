// WorkspaceService：Workspace 两层树（workspaces + workspace_projects 两表）的前端封装。
//
// 包 @bindings/xkpane workspace 服务（P3a）。后端是 cc-pane workspace_service 的
// SQLite 化重建：List 排序 = Pinned → SortOrder(NULL 垫底) → CreatedAt；
// 同一 workspace 内项目按归一化 path 去重 + 幂等；alias/path 设/清由后端 trim→NULL 归一。
// Reorder / ReorderProjects 供 @dnd-kit 拖拽排序消费（前端传"该组完整有序列表"）。
import { Service as Binding } from '@bindings/xkpane/internal/services/workspace';
import type {
  Workspace as RawWorkspace,
  WorkspaceProject as RawWorkspaceProject,
} from '@bindings/xkpane/internal/storage/models';

// WorkspaceProject 对外纯接口（树第二层节点）。path = PTY cwd / 操作目标（原样存储）。
export interface WorkspaceProject {
  id: string;
  workspaceId: string;
  path: string;
  alias: string | null; // null = 未设；显示名取 alias || basename(path)
  sortOrder: number;
}

// Workspace 对外纯接口（树第一层）。projects 已内联（List 预加载，按 sortOrder 排序）。
// createdAt/updatedAt 是后端 time.Time 序列化的 RFC3339 字符串。
export interface Workspace {
  id: string;
  name: string;
  alias: string | null;
  path: string | null; // 工作空间根目录；null = 未设
  providerId: string | null;
  pinned: boolean;
  sortOrder: number | null; // null 垫底
  projects: WorkspaceProject[];
  createdAt: string;
  updatedAt: string;
}

function toWorkspaceProject(p: RawWorkspaceProject): WorkspaceProject {
  return {
    id: p.id ?? '',
    workspaceId: p.workspaceId ?? '',
    path: p.path ?? '',
    alias: p.alias ?? null,
    sortOrder: typeof p.sortOrder === 'number' ? p.sortOrder : 0,
  };
}

function toWorkspace(w: RawWorkspace): Workspace {
  return {
    id: w.id ?? '',
    name: w.name ?? '',
    alias: w.alias ?? null,
    path: w.path ?? null,
    providerId: w.providerId ?? null,
    pinned: w.pinned ?? false,
    sortOrder: typeof w.sortOrder === 'number' ? w.sortOrder : null,
    projects: (w.projects ?? []).map(toWorkspaceProject),
    createdAt: typeof w.createdAt === 'string' ? w.createdAt : '',
    updatedAt: typeof w.updatedAt === 'string' ? w.updatedAt : '',
  };
}

export const WorkspaceService = {
  // 全部工作空间（含内联 projects），Pinned → SortOrder → CreatedAt 排序。
  async list(): Promise<Workspace[]> {
    const rows = await Binding.List();
    return (rows ?? []).map(toWorkspace);
  },

  // 新建工作空间（name 唯一；path 空白 → NULL）。重名抛 "Workspace already exists"。
  async create(name: string, path: string | null): Promise<Workspace> {
    return toWorkspace(await Binding.Create(name, path));
  },

  // 改显示名（空名/重名被后端拒绝）。
  async rename(id: string, name: string): Promise<void> {
    await Binding.Rename(id, name);
  },

  // 删除工作空间（级联删其下项目节点）。
  async remove(id: string): Promise<void> {
    await Binding.Delete(id);
  },

  // 设/清工作空间别名（null/空白清除）。
  async setAlias(id: string, alias: string | null): Promise<void> {
    await Binding.SetAlias(id, alias);
  },

  // 设/清工作空间根目录路径（null/空清除）。
  async setPath(id: string, path: string | null): Promise<void> {
    await Binding.SetPath(id, path);
  },

  // 置顶 / 取消置顶。
  async setPinned(id: string, pinned: boolean): Promise<void> {
    await Binding.SetPinned(id, pinned);
  },

  // 在工作空间下登记项目节点（归一化去重 + 幂等：已存在直接返回那条）。
  async addProject(workspaceId: string, path: string): Promise<WorkspaceProject> {
    return toWorkspaceProject(await Binding.AddProject(workspaceId, path));
  },

  // 删除项目节点。
  async removeProject(projectId: string): Promise<void> {
    await Binding.RemoveProject(projectId);
  },

  // 设/清项目节点别名（null/空白清除）。
  async setProjectAlias(projectId: string, alias: string | null): Promise<void> {
    await Binding.SetProjectAlias(projectId, alias);
  },

  // 拖拽排序：按给定顺序重写工作空间 sort_order（传该组完整有序 id 列表）。
  async reorder(ids: string[]): Promise<void> {
    await Binding.Reorder(ids);
  },

  // 拖拽排序：按给定顺序重写某工作空间下项目节点 sort_order。
  async reorderProjects(workspaceId: string, projectIds: string[]): Promise<void> {
    await Binding.ReorderProjects(workspaceId, projectIds);
  },
} as const;
