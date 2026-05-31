import { useCallback, useMemo, useRef, useState } from 'react';
import {
  LocalHistoryService,
  NativeDialogs,
  type DiffResult,
  type FileVersion,
  type HistoryLabel,
  type Project,
} from '@/services';
import { useProjects } from '@/project';

// HistoryPage ViewModel：消费全局 ProjectProvider 选项目 + 按 path 浏览本地历史（两阶段 + 4-tab）：
//   阶段 1 = 最近变更文件列表（GetRecentChanges 跨文件聚合，按文件去重保留最新）
//   阶段 2 = 版本时间线（倒序）+ 4-tab 预览（diff/content/deleted/project-restore）+ 分支筛选
//
// MVVM：不返回 JSX、不持翻译串（错误 / 确认标题由 View 经 t() 传入）。请求令牌防快速切换竞态。
// 忠实 cc-pane web/components/LocalHistoryPanel.tsx。

export type HistoryViewMode = 'diff' | 'content' | 'deleted' | 'project-restore';

// 阶段 1 列表的一行（按文件聚合后的最新变更）。
export interface HistoryFileEntry {
  filePath: string;
  fileName: string;
  dir: string;
  timestamp: string;
  size: number;
  branch: string;
  labelName: string;
}

// 恢复已删除文件 / 恢复到项目快照的文案束（View 经 t() 填好传入）。
export interface RestoreStrings {
  confirmTitle: string;
  confirmMessage: string; // 含 {{path}} / {{name}} / {{count}} 由 View 预先插值
  ok: string;
  cancel: string;
  successTitle: string;
  successMessage: string; // 含 {{count}} 由 View 预先插值
  errorTitle: string;
}

export interface UseHistoryPageResult {
  projects: Project[];
  selectedPath: string;
  loading: boolean;
  files: HistoryFileEntry[];
  selectedFile: string | null;
  // —— 阶段 2 ——
  versions: FileVersion[]; // 倒序（最新在上）
  filteredVersions: FileVersion[]; // 应用分支筛选后
  selectedVersion: FileVersion | null;
  viewMode: HistoryViewMode;
  fileBranches: string[];
  branchFilter: string; // '' = 全部分支
  versionLabels: Record<string, HistoryLabel[]>; // versionId → 关联标签
  diff: DiffResult | null;
  diffLoading: boolean;
  diffCaption: 'vsWorking' | 'betweenVersions' | null;
  content: string;
  contentLoading: boolean;
  deletedFiles: FileVersion[];
  deletedLoading: boolean;
  labels: HistoryLabel[];
  // —— 操作 ——
  select: (path: string, errorTitle: string) => Promise<void>;
  refresh: (errorTitle: string) => Promise<void>;
  selectFile: (filePath: string, errorTitle: string) => Promise<void>;
  backToList: () => void;
  selectVersion: (v: FileVersion, errorTitle: string) => Promise<void>;
  setViewMode: (mode: HistoryViewMode, errorTitle: string) => void;
  setBranchFilter: (branch: string, errorTitle: string) => void;
  restoreDeleted: (file: FileVersion, s: RestoreStrings) => Promise<void>;
  restoreToLabel: (label: HistoryLabel, s: RestoreStrings) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

function splitPath(filePath: string): { fileName: string; dir: string } {
  const p = filePath.replace(/\\/g, '/');
  const idx = p.lastIndexOf('/');
  if (idx < 0) return { fileName: p, dir: '' };
  return { fileName: p.slice(idx + 1), dir: p.slice(0, idx) };
}

function aggregateFiles(rows: Awaited<ReturnType<typeof LocalHistoryService.recentChanges>>): HistoryFileEntry[] {
  const seen = new Set<string>();
  const out: HistoryFileEntry[] = [];
  for (const r of rows) {
    if (seen.has(r.filePath)) continue;
    seen.add(r.filePath);
    const { fileName, dir } = splitPath(r.filePath);
    out.push({ filePath: r.filePath, fileName, dir, timestamp: r.timestamp, size: r.size, branch: r.branch, labelName: r.labelName });
  }
  return out;
}

export const useHistoryPage = (): UseHistoryPageResult => {
  const { projects } = useProjects();
  const [selectedPath, setSelectedPath] = useState('');
  const [loading, setLoading] = useState(false);
  const [files, setFiles] = useState<HistoryFileEntry[]>([]);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);

  // 阶段 2 状态
  const [versions, setVersions] = useState<FileVersion[]>([]);
  const [selectedVersion, setSelectedVersion] = useState<FileVersion | null>(null);
  const [viewMode, setViewModeState] = useState<HistoryViewMode>('diff');
  const [fileBranches, setFileBranches] = useState<string[]>([]);
  const [branchFilter, setBranchFilterState] = useState('');
  const [labels, setLabels] = useState<HistoryLabel[]>([]);
  const [diff, setDiff] = useState<DiffResult | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [diffCaption, setDiffCaption] = useState<'vsWorking' | 'betweenVersions' | null>(null);
  const [content, setContent] = useState('');
  const [contentLoading, setContentLoading] = useState(false);
  const [deletedFiles, setDeletedFiles] = useState<FileVersion[]>([]);
  const [deletedLoading, setDeletedLoading] = useState(false);

  // 请求令牌：丢弃过期响应（快速切项目 / 文件 / 版本）。
  const reqIdRef = useRef(0);
  const diffReqRef = useRef(0);
  // selectedPath 镜像：供 selectVersion 等回调读最新值而不进 deps。
  const pathRef = useRef('');
  pathRef.current = selectedPath;
  const fileRef = useRef<string | null>(null);
  fileRef.current = selectedFile;

  // 分支筛选后的版本列表（倒序保持）。
  const filteredVersions = useMemo(
    () => (branchFilter ? versions.filter((v) => v.branch === branchFilter) : versions),
    [versions, branchFilter],
  );

  // versionId → 关联标签（标签的 fileSnapshots 引用了当前文件的该版本）。
  const versionLabels = useMemo(() => {
    const map: Record<string, HistoryLabel[]> = {};
    if (!selectedFile) return map;
    for (const label of labels) {
      for (const snap of label.fileSnapshots) {
        if (snap.filePath === selectedFile) {
          (map[snap.versionId] ??= []).push(label);
        }
      }
    }
    return map;
  }, [labels, selectedFile]);

  const loadFiles = useCallback(async (path: string, errorTitle: string): Promise<void> => {
    if (!path) {
      setFiles([]);
      return;
    }
    const myReq = ++reqIdRef.current;
    setLoading(true);
    try {
      const rows = await LocalHistoryService.recentChanges(path, 200);
      if (myReq !== reqIdRef.current) return;
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
      await loadFiles(pathRef.current, errorTitle);
    },
    [loadFiles],
  );

  // —— 阶段 2：diff / content 加载（基于当前 filteredVersions 找相邻版本）——
  const loadDiff = useCallback(async (path: string, filePath: string, list: FileVersion[], v: FileVersion, errorTitle: string): Promise<void> => {
    const myReq = ++diffReqRef.current;
    setDiffLoading(true);
    try {
      const idx = list.findIndex((x) => x.id === v.id);
      const older = idx >= 0 ? list[idx + 1] : undefined; // 倒序：idx+1 更旧
      let result: DiffResult;
      let caption: 'vsWorking' | 'betweenVersions';
      if (older) {
        result = await LocalHistoryService.versionsDiff(path, filePath, older.id, v.id);
        caption = 'betweenVersions';
      } else {
        result = await LocalHistoryService.versionDiff(path, filePath, v.id); // 最早版 ↔ 工作树
        caption = 'vsWorking';
      }
      if (myReq !== diffReqRef.current) return;
      setDiff(result);
      setDiffCaption(caption);
    } catch (e) {
      if (myReq === diffReqRef.current) {
        setDiff(null);
        setDiffCaption(null);
        void NativeDialogs.error(errorTitle, messageOf(e));
      }
    } finally {
      if (myReq === diffReqRef.current) setDiffLoading(false);
    }
  }, []);

  const loadContent = useCallback(async (path: string, filePath: string, v: FileVersion, errorTitle: string): Promise<void> => {
    const myReq = ++diffReqRef.current;
    setContentLoading(true);
    try {
      const text = await LocalHistoryService.versionContent(path, filePath, v.id);
      if (myReq !== diffReqRef.current) return;
      setContent(text);
    } catch (e) {
      if (myReq === diffReqRef.current) {
        setContent('');
        void NativeDialogs.error(errorTitle, messageOf(e));
      }
    } finally {
      if (myReq === diffReqRef.current) setContentLoading(false);
    }
  }, []);

  const loadDeleted = useCallback(async (path: string, errorTitle: string): Promise<void> => {
    setDeletedLoading(true);
    try {
      setDeletedFiles(await LocalHistoryService.listDeletedFiles(path));
    } catch (e) {
      setDeletedFiles([]);
      void NativeDialogs.error(errorTitle, messageOf(e));
    } finally {
      setDeletedLoading(false);
    }
  }, []);

  // 选中一个版本：按当前 viewMode 加载 diff 或 content。
  const selectVersion = useCallback(
    async (v: FileVersion, errorTitle: string): Promise<void> => {
      setSelectedVersion(v);
      const path = pathRef.current;
      const filePath = fileRef.current;
      if (!filePath) return;
      if (viewMode === 'content') {
        await loadContent(path, filePath, v, errorTitle);
      } else {
        await loadDiff(path, filePath, filteredVersions, v, errorTitle);
      }
    },
    [viewMode, filteredVersions, loadDiff, loadContent],
  );

  // 进入阶段 2：加载版本 + 分支 + 标签，默认选最新版并按 diff 模式预览。
  const selectFile = useCallback(
    async (filePath: string, errorTitle: string): Promise<void> => {
      setSelectedFile(filePath);
      setViewModeState('diff');
      setBranchFilterState('');
      setSelectedVersion(null);
      setDiff(null);
      setContent('');
      const path = pathRef.current;
      try {
        const [vs, branches, lbls] = await Promise.all([
          LocalHistoryService.listVersions(path, filePath),
          LocalHistoryService.getFileBranches(path, filePath),
          LocalHistoryService.listLabels(path),
        ]);
        const ordered = [...vs].reverse(); // 最新在上
        setVersions(ordered);
        setFileBranches(branches);
        setLabels(lbls);
        const newest = ordered[0];
        if (newest) {
          setSelectedVersion(newest);
          await loadDiff(path, filePath, ordered, newest, errorTitle);
        }
      } catch (e) {
        setVersions([]);
        setFileBranches([]);
        void NativeDialogs.error(errorTitle, messageOf(e));
      }
    },
    [loadDiff],
  );

  const backToList = useCallback((): void => {
    setSelectedFile(null);
    setVersions([]);
    setSelectedVersion(null);
    setDeletedFiles([]);
  }, []);

  // 切换 tab：按需加载该 tab 数据（diff/content 复用选中版本；deleted 拉已删；project-restore 重拉标签）。
  const setViewMode = useCallback(
    (mode: HistoryViewMode, errorTitle: string): void => {
      setViewModeState(mode);
      const path = pathRef.current;
      const filePath = fileRef.current;
      if (mode === 'deleted') {
        void loadDeleted(path, errorTitle);
      } else if (mode === 'project-restore') {
        void LocalHistoryService.listLabels(path).then(setLabels).catch((e) => void NativeDialogs.error(errorTitle, messageOf(e)));
      } else if (mode === 'content' && selectedVersion && filePath) {
        void loadContent(path, filePath, selectedVersion, errorTitle);
      } else if (mode === 'diff' && selectedVersion && filePath) {
        void loadDiff(path, filePath, filteredVersions, selectedVersion, errorTitle);
      }
    },
    [selectedVersion, filteredVersions, loadDeleted, loadContent, loadDiff],
  );

  // 切分支筛选：若当前选中版本不在筛选结果内，改选筛选后的最新版并重载预览。
  const setBranchFilter = useCallback(
    (branch: string, errorTitle: string): void => {
      setBranchFilterState(branch);
      const path = pathRef.current;
      const filePath = fileRef.current;
      if (!filePath) return;
      const next = branch ? versions.filter((v) => v.branch === branch) : versions;
      const stillThere = selectedVersion && next.some((v) => v.id === selectedVersion.id);
      const target = stillThere ? selectedVersion : next[0] ?? null;
      setSelectedVersion(target);
      if (target) {
        if (viewMode === 'content') void loadContent(path, filePath, target, errorTitle);
        else void loadDiff(path, filePath, next, target, errorTitle);
      }
    },
    [versions, selectedVersion, viewMode, loadContent, loadDiff],
  );

  const restoreDeleted = useCallback(
    async (file: FileVersion, s: RestoreStrings): Promise<void> => {
      const ok = await NativeDialogs.confirm({ title: s.confirmTitle, message: s.confirmMessage, okLabel: s.ok, cancelLabel: s.cancel });
      if (!ok) return;
      try {
        await LocalHistoryService.restoreVersion(pathRef.current, file.filePath, file.id, file.branch);
        await loadDeleted(pathRef.current, s.errorTitle);
        await NativeDialogs.info(s.successTitle, s.successMessage);
      } catch (e) {
        void NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [loadDeleted],
  );

  const restoreToLabel = useCallback(async (label: HistoryLabel, s: RestoreStrings): Promise<void> => {
    const ok = await NativeDialogs.confirm({ title: s.confirmTitle, message: s.confirmMessage, okLabel: s.ok, cancelLabel: s.cancel });
    if (!ok) return;
    try {
      const restored = await LocalHistoryService.restoreToLabel(pathRef.current, label.id, label.branch);
      await NativeDialogs.info(s.successTitle, s.successMessage.replace('{{count}}', String(restored.length)));
    } catch (e) {
      void NativeDialogs.error(s.errorTitle, messageOf(e));
    }
  }, []);

  return {
    projects,
    selectedPath,
    loading,
    files,
    selectedFile,
    versions,
    filteredVersions,
    selectedVersion,
    viewMode,
    fileBranches,
    branchFilter,
    versionLabels,
    diff,
    diffLoading,
    diffCaption,
    content,
    contentLoading,
    deletedFiles,
    deletedLoading,
    labels,
    select,
    refresh,
    selectFile,
    backToList,
    selectVersion,
    setViewMode,
    setBranchFilter,
    restoreDeleted,
    restoreToLabel,
  };
};
