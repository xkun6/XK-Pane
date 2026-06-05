import type { Workspace, WorkspaceProject, SshProjectInfo } from '@/services';

// WorkspaceContextValue：Workspace 两层树的全局状态 + 动作（纯 Context，弃 Zustand）。
//
// 动作约定：调后端 → reload 刷新（与后端 Pinned→SortOrder→CreatedAt 排序、path 去重、
// alias trim 归一保持一致，不做乐观拼接以免与后端语义分裂）。错误**向上抛**，由 View 用
// NativeDialogs 提示（如 create 重名 → "Workspace already exists"），Provider 不吞错。
export interface WorkspaceContextValue {
  workspaces: Workspace[];
  loading: boolean; // 仅首屏拉取期为 true；mutation 刷新不切，避免树闪烁
  refresh: () => Promise<void>;
  create: (name: string, path: string | null) => Promise<Workspace>;
  rename: (id: string, name: string) => Promise<void>;
  remove: (id: string) => Promise<void>;
  setAlias: (id: string, alias: string | null) => Promise<void>;
  setPath: (id: string, path: string | null) => Promise<void>;
  setPinned: (id: string, pinned: boolean) => Promise<void>;
  addProject: (workspaceId: string, path: string) => Promise<WorkspaceProject>;
  addSSHProject: (workspaceId: string, cfg: SshProjectInfo) => Promise<WorkspaceProject>;
  removeProject: (projectId: string) => Promise<void>;
  setProjectAlias: (projectId: string, alias: string | null) => Promise<void>;
  reorder: (ids: string[]) => Promise<void>;
  reorderProjects: (workspaceId: string, projectIds: string[]) => Promise<void>;
}
