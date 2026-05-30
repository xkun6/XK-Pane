import { useCallback, useRef, useState } from 'react';
import { LocalHistoryService, NativeDialogs, type Project, type RecentChange } from '@/services';
import { useProjects } from '@/project';

// HistoryPage ViewModel：消费全局 ProjectProvider 选项目 + 按 path 查 Local History 最近变更，
// 支持手动快照与回滚。与 GitPage 同理 —— 历史是「按 path 即查」的视图态，不建全局 Provider，
// 状态留 VM（reqIdRef 请求令牌丢弃快速切换的过期响应）。不返回 JSX、不持翻译串（MVVM 铁律）。
//
// 本阶段（第一阶段）：最近变更列表 + 快照 + 回滚。diff 查看为后续单元。branch 一律传 ""
// （Local History 不依赖 git 分支，branch 仅元数据；接 gitsvc 取当前分支留后续）。

const RECENT_LIMIT = 100;

export interface SnapshotStrings {
  successTitle: string;
  createdMessage: string;
  noChangeMessage: string;
  errorTitle: string;
}

export interface RestoreStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  successTitle: string;
  successMessage: string;
  errorTitle: string;
}

export interface UseHistoryPageResult {
  projects: Project[];
  selectedPath: string;
  loading: boolean;
  busy: boolean;
  changes: RecentChange[];
  newFilePath: string;
  setNewFilePath: (v: string) => void;
  select: (path: string, errorTitle: string) => Promise<void>;
  refresh: (errorTitle: string) => Promise<void>;
  snapshot: (s: SnapshotStrings) => Promise<void>;
  restore: (c: RecentChange, s: RestoreStrings) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export const useHistoryPage = (): UseHistoryPageResult => {
  const { projects } = useProjects();
  const [selectedPath, setSelectedPath] = useState('');
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [changes, setChanges] = useState<RecentChange[]>([]);
  const [newFilePath, setNewFilePath] = useState('');

  const reqIdRef = useRef(0);

  const load = useCallback(async (path: string, errorTitle: string): Promise<void> => {
    const myReq = ++reqIdRef.current;
    setLoading(true);
    try {
      const rows = await LocalHistoryService.recentChanges(path, RECENT_LIMIT);
      if (myReq !== reqIdRef.current) return;
      setChanges(rows);
    } catch (e) {
      if (myReq === reqIdRef.current) setChanges([]);
      await NativeDialogs.error(errorTitle, messageOf(e));
    } finally {
      if (myReq === reqIdRef.current) setLoading(false);
    }
  }, []);

  const select = useCallback(
    async (path: string, errorTitle: string): Promise<void> => {
      setSelectedPath(path);
      if (!path) {
        reqIdRef.current++;
        setChanges([]);
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

  // 手动快照：读 newFilePath 指向的工作树文件存版本。空路径 / busy → 静默返回。
  // 内容未变（去重返回 null）→ 提示「无变化」；成功 → 清空输入并重拉列表。
  const snapshot = useCallback(
    async (s: SnapshotStrings): Promise<void> => {
      const rel = newFilePath.trim();
      if (!selectedPath || busy || !rel) return;
      setBusy(true);
      try {
        const v = await LocalHistoryService.snapshotFile(selectedPath, rel, '');
        if (v) {
          setNewFilePath('');
          await NativeDialogs.info(s.successTitle, s.createdMessage);
          await load(selectedPath, s.errorTitle);
        } else {
          await NativeDialogs.info(s.successTitle, s.noChangeMessage);
        }
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedPath, busy, newFilePath, load],
  );

  // 回滚：把某变更对应版本写回工作树（原生确认）。成功后重拉（Before-Restore 标签由后端打）。
  const restore = useCallback(
    async (c: RecentChange, s: RestoreStrings): Promise<void> => {
      if (!selectedPath || busy) return;
      const ok = await NativeDialogs.confirm({
        title: s.confirmTitle,
        message: s.confirmMessage,
        okLabel: s.okLabel,
        cancelLabel: s.cancelLabel,
      });
      if (!ok) return;
      setBusy(true);
      try {
        await LocalHistoryService.restoreVersion(selectedPath, c.filePath, c.versionId, '');
        await NativeDialogs.info(s.successTitle, s.successMessage);
        await load(selectedPath, s.errorTitle);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedPath, busy, load],
  );

  return {
    projects,
    selectedPath,
    loading,
    busy,
    changes,
    newFilePath,
    setNewFilePath,
    select,
    refresh,
    snapshot,
    restore,
  };
};
