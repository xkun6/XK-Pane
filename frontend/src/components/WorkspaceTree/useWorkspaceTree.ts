// WorkspaceTree 的 ViewModel：消费 WorkspaceContext + TerminalContext（启动）+ 编排原生对话框。
// 不返回 JSX、不持翻译串——提示文案由 View 传入（遵 MVVM 铁律）。
// 折叠状态为单选互斥（同一时刻最多展开一个工作空间），存组件本地 state（禁 localStorage）。
import { useState } from 'react';
import { FileService, NativeDialogs, type SshProjectInfo, type Workspace } from '@/services';
import { useWorkspaces } from '@/workspace';
import { useTerminal } from '@/terminal';
import { useRouter } from '@/router';
import { useCurrentDir } from '@/currentdir';

// 启动目标：工作空间节点用 ws.path / ws.id；项目节点用 project.path / project.id。
// projectId 落 LaunchHistory（软关联，不外键全局 projects 表）；path 为空（工作空间未设根目录）则不启动。
// ssh 非空 = SSH 远程项目：runtimeKind 走 'ssh'，path 为合成键（仅作标识，后端不当本地 cwd 用）。
export interface LaunchTarget {
  id: string;
  name: string;
  path: string | null;
  ssh?: SshProjectInfo | null;
}

export interface ConfirmStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

export interface ImportStrings {
  dialogTitle: string;
  errorTitle: string;
}

export interface WorkspaceTreeModel {
  loading: boolean;
  pinned: Workspace[];
  normal: Workspace[];
  isExpanded: (id: string) => boolean;
  toggleExpand: (id: string) => void;
  expand: (id: string) => void;
  submitCreate: (name: string, errorTitle: string) => Promise<void>;
  togglePinned: (ws: Workspace, errorTitle: string) => Promise<void>;
  deleteWorkspace: (ws: Workspace, s: ConfirmStrings) => Promise<void>;
  launch: (target: LaunchTarget, cliTool: string) => void;
  importProject: (workspaceId: string, s: ImportStrings) => Promise<void>;
  addSSHProject: (workspaceId: string, cfg: SshProjectInfo, errorTitle: string) => Promise<void>;
  submitRename: (id: string, name: string, errorTitle: string) => Promise<void>;
  submitAlias: (id: string, alias: string, errorTitle: string) => Promise<void>;
  submitProjectAlias: (projectId: string, alias: string, errorTitle: string) => Promise<void>;
  removeProject: (projectId: string, s: ConfirmStrings) => Promise<void>;
  copyPath: (path: string) => void;
  openInExplorer: (path: string, errorTitle: string) => Promise<void>;
  openInFileBrowser: (path: string) => void;
  reorderWorkspaces: (ids: string[], errorTitle: string) => Promise<void>;
  reorderProjects: (workspaceId: string, projectIds: string[], errorTitle: string) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export function useWorkspaceTree(): WorkspaceTreeModel {
  const ctx = useWorkspaces();
  const { launchProject } = useTerminal();
  const { navigate } = useRouter();
  const { openInBrowser } = useCurrentDir();
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const isExpanded = (id: string): boolean => expandedId === id;
  const toggleExpand = (id: string): void =>
    setExpandedId((prev) => (prev === id ? null : id));
  const expand = (id: string): void => setExpandedId(id);

  const submitCreate = async (name: string, errorTitle: string): Promise<void> => {
    try {
      await ctx.create(name, null);
    } catch (e) {
      await NativeDialogs.error(errorTitle, messageOf(e));
    }
  };

  const togglePinned = async (ws: Workspace, errorTitle: string): Promise<void> => {
    try {
      await ctx.setPinned(ws.id, !ws.pinned);
    } catch (e) {
      await NativeDialogs.error(errorTitle, messageOf(e));
    }
  };

  const deleteWorkspace = async (ws: Workspace, s: ConfirmStrings): Promise<void> => {
    const ok = await NativeDialogs.confirm({
      title: s.confirmTitle,
      message: s.confirmMessage,
      okLabel: s.okLabel,
      cancelLabel: s.cancelLabel,
    });
    if (!ok) return;
    try {
      await ctx.remove(ws.id);
    } catch (e) {
      await NativeDialogs.error(s.errorTitle, messageOf(e));
    }
  };

  // 启动 CLI/裸终端：path 为空（工作空间未设根目录）则静默忽略（View 已 disable 这些项）。
  // ssh 非空 → runtimeKind='ssh'，把连接信息透传后端（本地 ssh 客户端拉起远程会话）。
  const launch = (target: LaunchTarget, cliTool: string): void => {
    if (!target.path) return;
    launchProject({
      projectId: target.id,
      projectName: target.name,
      projectPath: target.path,
      cliTool,
      runtimeKind: target.ssh ? 'ssh' : undefined,
      ssh: target.ssh ?? undefined,
    });
  };

  const importProject = async (workspaceId: string, s: ImportStrings): Promise<void> => {
    const dir = await NativeDialogs.openDirectory({ title: s.dialogTitle });
    if (!dir) return;
    try {
      await ctx.addProject(workspaceId, dir);
    } catch (e) {
      await NativeDialogs.error(s.errorTitle, messageOf(e));
    }
  };

  // 在工作空间下登记 SSH 远程项目（cfg 已由 View 从所选机器 DTO 合成，不含密码）。
  const addSSHProject = async (
    workspaceId: string,
    cfg: SshProjectInfo,
    errorTitle: string,
  ): Promise<void> => {
    try {
      await ctx.addSSHProject(workspaceId, cfg);
    } catch (e) {
      await NativeDialogs.error(errorTitle, messageOf(e));
    }
  };

  const submitRename = async (id: string, name: string, errorTitle: string): Promise<void> => {
    try {
      await ctx.rename(id, name);
    } catch (e) {
      await NativeDialogs.error(errorTitle, messageOf(e));
    }
  };

  // alias 传空串 → 后端归一为 NULL（清除别名）。
  const submitAlias = async (id: string, alias: string, errorTitle: string): Promise<void> => {
    try {
      await ctx.setAlias(id, alias);
    } catch (e) {
      await NativeDialogs.error(errorTitle, messageOf(e));
    }
  };

  const submitProjectAlias = async (
    projectId: string,
    alias: string,
    errorTitle: string,
  ): Promise<void> => {
    try {
      await ctx.setProjectAlias(projectId, alias);
    } catch (e) {
      await NativeDialogs.error(errorTitle, messageOf(e));
    }
  };

  const removeProject = async (projectId: string, s: ConfirmStrings): Promise<void> => {
    const ok = await NativeDialogs.confirm({
      title: s.confirmTitle,
      message: s.confirmMessage,
      okLabel: s.okLabel,
      cancelLabel: s.cancelLabel,
    });
    if (!ok) return;
    try {
      await ctx.removeProject(projectId);
    } catch (e) {
      await NativeDialogs.error(s.errorTitle, messageOf(e));
    }
  };

  const copyPath = (path: string): void => {
    void navigator.clipboard?.writeText(path).catch(() => {});
  };

  // 在系统文件管理器中定位该目录（Windows explorer / macOS Finder / Linux xdg-open）。失败弹错。
  const openInExplorer = async (path: string, errorTitle: string): Promise<void> => {
    try {
      await FileService.revealInOS(path);
    } catch (e) {
      await NativeDialogs.error(errorTitle, messageOf(e));
    }
  };

  // 在 XK-Pane 文件页打开该目录：设为文件浏览器树根（同时绑定 Git/历史抽屉），跳到文件页。
  const openInFileBrowser = (path: string): void => {
    openInBrowser(path);
    navigate('files');
  };

  // 拖拽排序：前端传该组/该工作空间的完整有序 id 列表，后端逐个写 sort_order=位置。
  const reorderWorkspaces = async (ids: string[], errorTitle: string): Promise<void> => {
    try {
      await ctx.reorder(ids);
    } catch (e) {
      await NativeDialogs.error(errorTitle, messageOf(e));
    }
  };
  const reorderProjects = async (
    workspaceId: string,
    projectIds: string[],
    errorTitle: string,
  ): Promise<void> => {
    try {
      await ctx.reorderProjects(workspaceId, projectIds);
    } catch (e) {
      await NativeDialogs.error(errorTitle, messageOf(e));
    }
  };

  return {
    loading: ctx.loading,
    pinned: ctx.workspaces.filter((w) => w.pinned),
    normal: ctx.workspaces.filter((w) => !w.pinned),
    isExpanded,
    toggleExpand,
    expand,
    submitCreate,
    togglePinned,
    deleteWorkspace,
    launch,
    importProject,
    addSSHProject,
    submitRename,
    submitAlias,
    submitProjectAlias,
    removeProject,
    copyPath,
    openInExplorer,
    openInFileBrowser,
    reorderWorkspaces,
    reorderProjects,
  };
}
