import { useCallback, useRef, useState } from 'react';
import { LocalHistoryService, NativeDialogs, type Project } from '@/services';
import { useProjects } from '@/project';

// HistoryPage ViewModel：消费全局 ProjectProvider 选项目 + 按 path 浏览本地历史（两阶段）：
//   阶段 1 = 最近变更文件列表（GetRecentChanges 跨文件聚合，按文件去重保留最新）
//   阶段 2 = 版本时间线 + 4-tab 预览（P4c-2/3）
//
// MVVM：不返回 JSX、不持翻译串（错误 / 确认标题由 View 经 t() 传入）。请求令牌防快速切换竞态。

// 阶段 1 列表的一行（按文件聚合后的最新变更）。
export interface HistoryFileEntry {
  filePath: string; // 项目内相对路径
  fileName: string; // basename
  dir: string; // 目录部分（空 = 根）
  timestamp: string; // 最新版本时间（RFC3339）
  size: number;
  branch: string;
  labelName: string;
}

export interface UseHistoryPageResult {
  projects: Project[];
  selectedPath: string;
  loading: boolean;
  files: HistoryFileEntry[];
  selectedFile: string | null;
  select: (path: string, errorTitle: string) => Promise<void>;
  refresh: (errorTitle: string) => Promise<void>;
  selectFile: (filePath: string) => void;
  backToList: () => void;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

// 把相对路径拆成 basename + 目录部分（统一正斜杠）。
function splitPath(filePath: string): { fileName: string; dir: string } {
  const p = filePath.replace(/\\/g, '/');
  const idx = p.lastIndexOf('/');
  if (idx < 0) return { fileName: p, dir: '' };
  return { fileName: p.slice(idx + 1), dir: p.slice(0, idx) };
}

// 把 GetRecentChanges（跨文件、最新在前）按文件去重，保留每文件最新一条。
function aggregateFiles(rows: Awaited<ReturnType<typeof LocalHistoryService.recentChanges>>): HistoryFileEntry[] {
  const seen = new Set<string>();
  const out: HistoryFileEntry[] = [];
  for (const r of rows) {
    if (seen.has(r.filePath)) continue;
    seen.add(r.filePath);
    const { fileName, dir } = splitPath(r.filePath);
    out.push({
      filePath: r.filePath,
      fileName,
      dir,
      timestamp: r.timestamp,
      size: r.size,
      branch: r.branch,
      labelName: r.labelName,
    });
  }
  return out;
}

export const useHistoryPage = (): UseHistoryPageResult => {
  const { projects } = useProjects();
  const [selectedPath, setSelectedPath] = useState('');
  const [loading, setLoading] = useState(false);
  const [files, setFiles] = useState<HistoryFileEntry[]>([]);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);

  // 请求令牌：快速切换项目时丢弃过期响应。
  const reqIdRef = useRef(0);

  const loadFiles = useCallback(async (path: string, errorTitle: string): Promise<void> => {
    if (!path) {
      setFiles([]);
      return;
    }
    const myReq = ++reqIdRef.current;
    setLoading(true);
    try {
      const rows = await LocalHistoryService.recentChanges(path, 200);
      if (myReq !== reqIdRef.current) return; // 过期
      setFiles(aggregateFiles(rows));
    } catch (e) {
      if (myReq === reqIdRef.current) {
        setFiles([]);
        void NativeDialogs.error(errorTitle, messageOf(e));
      }
    } finally {
      if (myReq === reqIdRef.current) setLoading(false);
    }
  }, []);

  const select = useCallback(
    async (path: string, errorTitle: string): Promise<void> => {
      setSelectedPath(path);
      setSelectedFile(null);
      await loadFiles(path, errorTitle);
    },
    [loadFiles],
  );

  const refresh = useCallback(
    async (errorTitle: string): Promise<void> => {
      await loadFiles(selectedPath, errorTitle);
    },
    [loadFiles, selectedPath],
  );

  const selectFile = useCallback((filePath: string): void => {
    setSelectedFile(filePath);
  }, []);

  const backToList = useCallback((): void => {
    setSelectedFile(null);
  }, []);

  return {
    projects,
    selectedPath,
    loading,
    files,
    selectedFile,
    select,
    refresh,
    selectFile,
    backToList,
  };
};
