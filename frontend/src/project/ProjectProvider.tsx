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
import { ProjectService, type Project } from '@/services';
import type { ProjectContextValue } from './types';

const ProjectContext = createContext<ProjectContextValue | null>(null);

// 项目登记表全局 Provider（SQLite 后端，弃 Zustand）。范式同 PreferencesProvider：
// 首屏空列表立即渲染，mount 异步拉取；mutation 调后端后 reload 刷新（与后端排序/
// 别名归一对齐，不做乐观拼接）。错误不吞——向上抛给调用方（View 用 NativeDialogs 提示）。
export interface ProjectProviderProps {
  children: ReactNode;
}

export const ProjectProvider = ({ children }: ProjectProviderProps) => {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);

  // 防止 unmount 后再 setState。
  const aliveRef = useRef(true);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  // 从后端拉取列表（最新在前）。供 mount 初始化与每次 mutation 后刷新复用；
  // 不切 loading（避免 mutation 时列表闪烁）。
  const reload = useCallback(async (): Promise<void> => {
    const rows = await ProjectService.list();
    if (aliveRef.current) setProjects(rows);
  }, []);

  // mount 拉取一次（带 loading；失败保持空列表，不阻断 UI）。
  useEffect(() => {
    let cancelled = false;
    ProjectService.list()
      .then((rows) => {
        if (!cancelled && aliveRef.current) setProjects(rows);
      })
      .catch(() => {
        // 后端未就绪 / 异常 —— 保持空列表
      })
      .finally(() => {
        if (!cancelled && aliveRef.current) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const refresh = useCallback((): Promise<void> => reload(), [reload]);

  const addProject = useCallback(
    async (path: string): Promise<Project> => {
      const created = await ProjectService.add(path);
      await reload();
      return created;
    },
    [reload],
  );

  const removeProject = useCallback(
    async (id: string): Promise<void> => {
      await ProjectService.remove(id);
      await reload();
    },
    [reload],
  );

  const renameProject = useCallback(
    async (id: string, name: string): Promise<void> => {
      await ProjectService.updateName(id, name);
      await reload();
    },
    [reload],
  );

  const setProjectAlias = useCallback(
    async (id: string, alias: string | null): Promise<void> => {
      await ProjectService.updateAlias(id, alias);
      await reload();
    },
    [reload],
  );

  const value = useMemo<ProjectContextValue>(
    () => ({ projects, loading, refresh, addProject, removeProject, renameProject, setProjectAlias }),
    [projects, loading, refresh, addProject, removeProject, renameProject, setProjectAlias],
  );

  return <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>;
};

export const useProjects = (): ProjectContextValue => {
  const ctx = useContext(ProjectContext);
  if (!ctx) {
    throw new Error('useProjects must be used inside <ProjectProvider>');
  }
  return ctx;
};
