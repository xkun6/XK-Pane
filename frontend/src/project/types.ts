import type { Project } from '@/services';

// ProjectContextValue：扁平项目登记表的全局状态 + 动作（弃 Zustand，纯 Context）。
//
// 动作约定：调后端 → 刷新列表（与后端 created_at DESC 排序、alias trim 归一保持
// 一致，不做乐观拼接以免与后端语义分裂）。错误**向上抛**，由 View 用
// NativeDialogs 提示（如 add 重复路径 → "Project already exists"），Provider 不吞错。
export interface ProjectContextValue {
  projects: Project[];
  loading: boolean; // 仅首屏拉取期为 true；mutation 刷新不切，避免列表闪烁
  refresh: () => Promise<void>;
  addProject: (path: string) => Promise<Project>;
  removeProject: (id: string) => Promise<void>;
  renameProject: (id: string, name: string) => Promise<void>;
  setProjectAlias: (id: string, alias: string | null) => Promise<void>;
}
