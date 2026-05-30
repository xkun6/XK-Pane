// WorkspaceTree 的 ViewModel：消费 WorkspaceContext + TerminalContext（启动）+ 编排原生对话框。
// 不返回 JSX、不持翻译串——提示文案由 View 传入（遵 MVVM 铁律）。
// 折叠状态为单选互斥（同一时刻最多展开一个工作空间），存组件本地 state（禁 localStorage）。
import { useState } from 'react';
import { NativeDialogs, type Workspace } from '@/services';
import { useWorkspaces } from '@/workspace';
import { useTerminal } from '@/terminal';

// 启动目标：工作空间节点用 ws.path / ws.id；项目节点用 project.path / project.id。
// projectId 落 LaunchHistory（软关联，不外键全局 projects 表）；path 为空（工作空间未设根目录）则不启动。
export interface LaunchTarget {
  id: string;
  name: string;
  path: string | null;
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
  submitRename: (id: string, name: string, errorTitle: string) => Promise<void>;
  submitAlias: (id: string, alias: string, errorTitle: string) => Promise<void>;
  submitProjectAlias: (projectId: string, alias: string, errorTitle: string) => Promise<void>;
  removeProject: (projectId: string, s: ConfirmStrings) => Promise<void>;
  copyPath: (path: string) => void;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export function useWorkspaceTree(): WorkspaceTreeModel {
  const ctx = useWorkspaces();
  const { launchProject } = useTerminal();
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
  const launch = (target: LaunchTarget, cliTool: string): void => {
    if (!target.path) return;
    launchProject({
      projectId: target.id,
      projectName: target.name,
      projectPath: target.path,
      cliTool,
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
    submitRename,
    submitAlias,
    submitProjectAlias,
    removeProject,
    copyPath,
  };
}
