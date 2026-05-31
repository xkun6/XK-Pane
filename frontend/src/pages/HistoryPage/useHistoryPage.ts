import { useCallback, useMemo, useRef, useState } from 'react';
import {
  LocalHistoryService,
  NativeDialogs,
  type DiffResult,
  type FileVersion,
  type HistoryConfig,
  type HistoryLabel,
  type HistoryStats,
  type Project,
} from '@/services';
import { useProjects } from '@/project';

// HistoryPage ViewModel：消费全局 ProjectProvider 选项目 + 按 path 浏览本地历史（两阶段 + 4-tab）+
// 标签 / 手动快照 / 单文件回滚 / 配置 / 统计 / 清理 + watcher live 刷新。
//
// MVVM：不返回 JSX、不持翻译串（错误 / 确认标题由 View 经 t() 传入）。请求令牌防快速切换竞态。
// 忠实 cc-pane web/components/LocalHistoryPanel.tsx。

export type HistoryViewMode = 'diff' | 'content' | 'deleted' | 'project-restore';

export interface HistoryFileEntry {
  filePath: string;
  fileName: string;
  dir: string;
  timestamp: string;
  size: number;
  branch: string;
  labelName: string;
}

// 恢复（已删除文件 / 项目快照）文案束。
export interface RestoreStrings {
  confirmTitle: string;
  confirmMessage: string;
  ok: string;
  cancel: string;
  successTitle: string;
  successMessage: string;
  errorTitle: string;
}

// 打标签结果文案束（输入框由 View 的 TextInputDialog 提供）。
export interface TagStrings {
  successTitle: string;
  successMessage: string;
  errorTitle: string;
}

// 手动快照文案束。
export interface SnapshotStrings {
  createdTitle: string;
  createdMessage: string;
  noChangeMessage: string;
  errorTitle: string;
}

// 保存配置文案束。
export interface ConfigStrings {
  successTitle: string;
  successMessage: string;
  errorTitle: string;
}

// 立即清理文案束。
export interface CleanupStrings {
  confirmTitle: string;
  confirmMessage: string;
  ok: string;
  cancel: string;
  doneTitle: string;
  doneMessage: string;
  errorTitle: string;
}

export interface UseHistoryPageResult {
  projects: Project[];
  selectedPath: string;
  loading: boolean;
  files: HistoryFileEntry[];
  selectedFile: string | null;
  versions: FileVersion[];
  filteredVersions: FileVersion[];
  selectedVersion: FileVersion | null;
  viewMode: HistoryViewMode;
  fileBranches: string[];
  branchFilter: string;
  versionLabels: Record<string, HistoryLabel[]>;
  diff: DiffResult | null;
  diffLoading: boolean;
  diffCaption: 'vsWorking' | 'betweenVersions' | null;
  content: string;
  contentLoading: boolean;
  deletedFiles: FileVersion[];
  deletedLoading: boolean;
  labels: HistoryLabel[];
  config: HistoryConfig | null;
  stats: HistoryStats | null;
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
  restoreVersion: (version: FileVersion, s: RestoreStrings) => Promise<void>;
  addTag: (version: FileVersion, name: string, s: TagStrings) => Promise<void>;
  snapshot: (relPath: string, s: SnapshotStrings) => Promise<void>;
  setConfig: (cfg: HistoryConfig) => void;
  saveConfig: (s: ConfigStrings) => Promise<void>;
  cleanup: (s: CleanupStrings) => Promise<void>;
  reloadSilent: () => void; // watcher live 刷新（无错误弹窗）
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
  const [config, setConfigState] = useState<HistoryConfig | null>(null);
  const [stats, setStats] = useState<HistoryStats | null>(null);

  const reqIdRef = useRef(0);
  const diffReqRef = useRef(0);
  const pathRef = useRef('');
  pathRef.current = selectedPath;
  const fileRef = useRef<string | null>(null);
  fileRef.current = selectedFile;

  const filteredVersions = useMemo(
    () => (branchFilter ? versions.filter((v) => v.branch === branchFilter) : versions),
    [versions, branchFilter],
  );

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

  const loadConfigStats = useCallback(async (path: string): Promise<void> => {
    try {
      const [cfg, st] = await Promise.all([LocalHistoryService.getConfig(path), LocalHistoryService.getStats(path)]);
      setConfigState(cfg);
      setStats(st);
    } catch {
      // 配置 / 统计加载失败不阻断列表（兜底 null），不弹窗。
      setConfigState(null);
      setStats(null);
    }
  }, []);

  const select = useCallback(
    async (path: string, errorTitle: string): Promise<void> => {
      setSelectedPath(path);
      setSelectedFile(null);
      await Promise.all([loadFiles(path, errorTitle), loadConfigStats(path)]);
    },
    [loadFiles, loadConfigStats],
  );

  const refresh = useCallback(
    async (errorTitle: string): Promise<void> => {
      await Promise.all([loadFiles(pathRef.current, errorTitle), loadConfigStats(pathRef.current)]);
    },
    [loadFiles, loadConfigStats],
  );

  const loadDiff = useCallback(async (path: string, filePath: string, list: FileVersion[], v: FileVersion, errorTitle: string): Promise<void> => {
    const myReq = ++diffReqRef.current;
    setDiffLoading(true);
    try {
      const idx = list.findIndex((x) => x.id === v.id);
      const older = idx >= 0 ? list[idx + 1] : undefined;
      let result: DiffResult;
      let caption: 'vsWorking' | 'betweenVersions';
      if (older) {
        result = await LocalHistoryService.versionsDiff(path, filePath, older.id, v.id);
        caption = 'betweenVersions';
      } else {
        result = await LocalHistoryService.versionDiff(path, filePath, v.id);
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

  // 加载某文件的版本 + 分支 + 标签（供 selectFile 与 reloadSilent 共用）。preserveId 用于 live 刷新
  // 时尽量保留当前选中版本；返回选中的版本（供后续加载预览）。
  const loadVersionsFor = useCallback(
    async (path: string, filePath: string, preserveId: string | null): Promise<{ ordered: FileVersion[]; selected: FileVersion | null }> => {
      const [vs, branches, lbls] = await Promise.all([
        LocalHistoryService.listVersions(path, filePath),
        LocalHistoryService.getFileBranches(path, filePath),
        LocalHistoryService.listLabels(path),
      ]);
      const ordered = [...vs].reverse();
      setVersions(ordered);
      setFileBranches(branches);
      setLabels(lbls);
      const kept = preserveId ? ordered.find((v) => v.id === preserveId) : undefined;
      const selected = kept ?? ordered[0] ?? null;
      setSelectedVersion(selected);
      return { ordered, selected };
    },
    [],
  );

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
        const { ordered, selected } = await loadVersionsFor(path, filePath, null);
        if (selected) await loadDiff(path, filePath, ordered, selected, errorTitle);
      } catch (e) {
        setVersions([]);
        setFileBranches([]);
        void NativeDialogs.error(errorTitle, messageOf(e));
      }
    },
    [loadVersionsFor, loadDiff],
  );

  const backToList = useCallback((): void => {
    setSelectedFile(null);
    setVersions([]);
    setSelectedVersion(null);
    setDeletedFiles([]);
  }, []);

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

  // 单文件回滚到选中版本（diff/content tab 的「回滚到此版本」），恢复前后端自动打 Before-Restore 标签。
  const restoreVersion = useCallback(
    async (version: FileVersion, s: RestoreStrings): Promise<void> => {
      const filePath = fileRef.current;
      if (!filePath) return;
      const ok = await NativeDialogs.confirm({ title: s.confirmTitle, message: s.confirmMessage, okLabel: s.ok, cancelLabel: s.cancel });
      if (!ok) return;
      const path = pathRef.current;
      try {
        await LocalHistoryService.restoreVersion(path, filePath, version.id, version.branch);
        await loadVersionsFor(path, filePath, version.id);
        await loadFiles(path, s.errorTitle);
        await NativeDialogs.info(s.successTitle, s.successMessage);
      } catch (e) {
        void NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [loadVersionsFor, loadFiles],
  );

  // 为某版本打标签（source='user' / type='manual' / 单文件快照）。成功后重拉标签刷新 badge。
  const addTag = useCallback(
    async (version: FileVersion, name: string, s: TagStrings): Promise<void> => {
      const filePath = fileRef.current;
      const path = pathRef.current;
      if (!filePath) return;
      try {
        await LocalHistoryService.putLabel(path, {
          id: crypto.randomUUID(),
          name,
          labelType: 'manual',
          source: 'user',
          timestamp: new Date().toISOString(),
          branch: version.branch,
          fileSnapshots: [{ filePath, versionId: version.id }],
        });
        setLabels(await LocalHistoryService.listLabels(path));
        await NativeDialogs.info(s.successTitle, s.successMessage);
      } catch (e) {
        void NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [],
  );

  // 手动快照一个项目内相对路径文件（无节流；内容未变因去重返回 null）。
  const snapshot = useCallback(
    async (relPath: string, s: SnapshotStrings): Promise<void> => {
      const path = pathRef.current;
      const rel = relPath.trim();
      if (!path || !rel) return;
      try {
        const v = await LocalHistoryService.snapshotFile(path, rel, '');
        await loadFiles(path, s.errorTitle);
        if (rel === fileRef.current) await loadVersionsFor(path, rel, null);
        if (v) await NativeDialogs.info(s.createdTitle, s.createdMessage);
        else await NativeDialogs.info(s.createdTitle, s.noChangeMessage);
      } catch (e) {
        void NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [loadFiles, loadVersionsFor],
  );

  const setConfig = useCallback((cfg: HistoryConfig): void => {
    setConfigState(cfg);
  }, []);

  const saveConfig = useCallback(
    async (s: ConfigStrings): Promise<void> => {
      const path = pathRef.current;
      if (!path || !config) return;
      try {
        await LocalHistoryService.saveConfig(path, config);
        await loadConfigStats(path);
        await NativeDialogs.info(s.successTitle, s.successMessage);
      } catch (e) {
        void NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [config, loadConfigStats],
  );

  const cleanup = useCallback(
    async (s: CleanupStrings): Promise<void> => {
      const path = pathRef.current;
      if (!path) return;
      const ok = await NativeDialogs.confirm({ title: s.confirmTitle, message: s.confirmMessage, okLabel: s.ok, cancelLabel: s.cancel });
      if (!ok) return;
      try {
        await LocalHistoryService.cleanup(path);
        await Promise.all([loadConfigStats(path), loadFiles(path, s.errorTitle)]);
        await NativeDialogs.info(s.doneTitle, s.doneMessage);
      } catch (e) {
        void NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [loadConfigStats, loadFiles],
  );

  // watcher live 刷新（无错误弹窗）：静默重拉文件列表，若在阶段 2 则重拉版本（保留当前选中版本）。
  const reloadSilent = useCallback((): void => {
    const path = pathRef.current;
    if (!path) return;
    void (async () => {
      try {
        const rows = await LocalHistoryService.recentChanges(path, 200);
        setFiles(aggregateFiles(rows));
        const filePath = fileRef.current;
        if (filePath) {
          await loadVersionsFor(path, filePath, selectedVersion?.id ?? null);
        }
        await loadConfigStats(path);
      } catch {
        // live 刷新静默失败
      }
    })();
  }, [loadVersionsFor, loadConfigStats, selectedVersion]);

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
    config,
    stats,
    select,
    refresh,
    selectFile,
    backToList,
    selectVersion,
    setViewMode,
    setBranchFilter,
    restoreDeleted,
    restoreToLabel,
    restoreVersion,
    addTag,
    snapshot,
    setConfig,
    saveConfig,
    cleanup,
    reloadSilent,
  };
};
