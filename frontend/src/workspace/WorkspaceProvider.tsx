import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { WorkspaceService, type Workspace, type WorkspaceProject } from '@/services';
import type { WorkspaceContextValue } from './types';

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null);

// Workspace 两层树全局 Provider（SQLite 后端，弃 Zustand）。范式同 ProjectProvider：
// 首屏空树立即渲染，mount 异步拉取；mutation 调后端后 reload 刷新（与后端排序/去重/
// 别名归一对齐，不做乐观拼接）。错误不吞——向上抛给调用方（View 用 NativeDialogs 提示）。
export interface WorkspaceProviderProps {
  children: ReactNode;
}

export const WorkspaceProvider = ({ children }: WorkspaceProviderProps) => {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [loading, setLoading] = useState(true);

  // 防止 unmount 后再 setState。
  const aliveRef = useRef(true);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  // 从后端拉取整棵树。供 mount 初始化与每次 mutation 后刷新复用；不切 loading
  // （避免 mutation 时树闪烁）。
  const reload = useCallback(async (): Promise<void> => {
    const rows = await WorkspaceService.list();
    if (aliveRef.current) setWorkspaces(rows);
  }, []);

  // mount 拉取一次（带 loading；失败保持空树，不阻断 UI）。
  useEffect(() => {
    let cancelled = false;
    WorkspaceService.list()
      .then((rows) => {
        if (!cancelled && aliveRef.current) setWorkspaces(rows);
      })
      .catch(() => {
        // 后端未就绪 / 异常 —— 保持空树
      })
      .finally(() => {
        if (!cancelled && aliveRef.current) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const refresh = useCallback((): Promise<void> => reload(), [reload]);

  const create = useCallback(
    async (name: string, path: string | null): Promise<Workspace> => {
      const created = await WorkspaceService.create(name, path);
      await reload();
      return created;
    },
    [reload],
  );

  const rename = useCallback(
    async (id: string, name: string): Promise<void> => {
      await WorkspaceService.rename(id, name);
      await reload();
    },
    [reload],
  );

  const remove = useCallback(
    async (id: string): Promise<void> => {
      await WorkspaceService.remove(id);
      await reload();
    },
    [reload],
  );

  const setAlias = useCallback(
    async (id: string, alias: string | null): Promise<void> => {
      await WorkspaceService.setAlias(id, alias);
      await reload();
    },
    [reload],
  );

  const setPath = useCallback(
    async (id: string, path: string | null): Promise<void> => {
      await WorkspaceService.setPath(id, path);
      await reload();
    },
    [reload],
  );

  const setPinned = useCallback(
    async (id: string, pinned: boolean): Promise<void> => {
      await WorkspaceService.setPinned(id, pinned);
      await reload();
    },
    [reload],
  );

  const addProject = useCallback(
    async (workspaceId: string, path: string): Promise<WorkspaceProject> => {
      const created = await WorkspaceService.addProject(workspaceId, path);
      await reload();
      return created;
    },
    [reload],
  );

  const removeProject = useCallback(
    async (projectId: string): Promise<void> => {
      await WorkspaceService.removeProject(projectId);
      await reload();
    },
    [reload],
  );

  const setProjectAlias = useCallback(
    async (projectId: string, alias: string | null): Promise<void> => {
      await WorkspaceService.setProjectAlias(projectId, alias);
      await reload();
    },
    [reload],
  );

  const reorder = useCallback(
    async (ids: string[]): Promise<void> => {
      await WorkspaceService.reorder(ids);
      await reload();
    },
    [reload],
  );

  const reorderProjects = useCallback(
    async (workspaceId: string, projectIds: string[]): Promise<void> => {
      await WorkspaceService.reorderProjects(workspaceId, projectIds);
      await reload();
    },
    [reload],
  );

  const value = useMemo<WorkspaceContextValue>(
    () => ({
      workspaces,
      loading,
      refresh,
      create,
      rename,
      remove,
      setAlias,
      setPath,
      setPinned,
      addProject,
      removeProject,
      setProjectAlias,
      reorder,
      reorderProjects,
    }),
    [
      workspaces,
      loading,
      refresh,
      create,
      rename,
      remove,
      setAlias,
      setPath,
      setPinned,
      addProject,
      removeProject,
      setProjectAlias,
      reorder,
      reorderProjects,
    ],
  );

  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>;
};

export const useWorkspaces = (): WorkspaceContextValue => {
  const ctx = useContext(WorkspaceContext);
  if (!ctx) {
    throw new Error('useWorkspaces must be used inside <WorkspaceProvider>');
  }
  return ctx;
};
