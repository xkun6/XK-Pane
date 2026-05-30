import { useCallback, useRef, useState } from 'react';
import { GitService, NativeDialogs, type GitBranch, type GitCommit, type GitStatus, type Project } from '@/services';
import { useProjects } from '@/project';

// GitPage ViewModel：消费全局 ProjectProvider 选项目 + 按 path 查询只读 Git 状态。
//
// 为何不建全局 GitProvider：Git 状态是**按 path 即查**的视图态（非跨页面共享的全局登记表），
// 与 project/provider/todo 那类「全局列表」性质不同 —— 建 Provider 是投机抽象（违反简洁优先）。
// 故状态留在本 VM；若将来侧边栏需 per-project Git 角标等多消费方，再提升为 Provider。
//
// 不返回 JSX、不持翻译串（错误标题由 View 经 t() 传入，MVVM 铁律）。

const LOG_LIMIT = 20;

export interface UseGitPageResult {
  projects: Project[];
  selectedPath: string;
  loading: boolean;
  status: GitStatus | null;
  branches: GitBranch[];
  commits: GitCommit[];
  select: (path: string, errorTitle: string) => Promise<void>;
  refresh: (errorTitle: string) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export const useGitPage = (): UseGitPageResult => {
  const { projects } = useProjects();
  const [selectedPath, setSelectedPath] = useState('');
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<GitStatus | null>(null);
  const [branches, setBranches] = useState<GitBranch[]>([]);
  const [commits, setCommits] = useState<GitCommit[]>([]);

  // 请求令牌：快速切换项目时丢弃过期响应，只采纳最新一次。
  const reqIdRef = useRef(0);

  const load = useCallback(async (path: string, errorTitle: string): Promise<void> => {
    const myReq = ++reqIdRef.current;
    setLoading(true);
    try {
      const st = await GitService.getStatus(path);
      if (myReq !== reqIdRef.current) return; // 已被更新的请求取代
      setStatus(st);
      if (st.isRepo) {
        const [brs, logs] = await Promise.all([GitService.branches(path), GitService.log(path, LOG_LIMIT)]);
        if (myReq !== reqIdRef.current) return;
        setBranches(brs);
        setCommits(logs);
      } else {
        setBranches([]);
        setCommits([]);
      }
    } catch (e) {
      if (myReq === reqIdRef.current) {
        setStatus(null);
        setBranches([]);
        setCommits([]);
      }
      await NativeDialogs.error(errorTitle, messageOf(e));
    } finally {
      if (myReq === reqIdRef.current) setLoading(false);
    }
  }, []);

  const select = useCallback(
    async (path: string, errorTitle: string): Promise<void> => {
      setSelectedPath(path);
      if (!path) {
        reqIdRef.current++; // 取消在途请求
        setStatus(null);
        setBranches([]);
        setCommits([]);
        return;
      }
      await load(path, errorTitle);
    },
    [load],
  );

  const refresh = useCallback(
    async (errorTitle: string): Promise<void> => {
      if (!selectedPath) return;
      await load(selectedPath, errorTitle);
    },
    [selectedPath, load],
  );

  return { projects, selectedPath, loading, status, branches, commits, select, refresh };
};
