import { useCallback, useRef, useState } from 'react';
import { NativeDialogs, SpecService, type Project, type Spec } from '@/services';
import { useProjects } from '@/project';

// SpecsPage ViewModel：消费全局 ProjectProvider 选项目 + 按 path 管理该项目的 Spec
// （列表 / 新建 / 选中看-编辑 .md 内容 / 激活 / 归档 / 删除）。与 GitPage·HistoryPage 同理：
// 「按 path 即查」的视图态，不建全局 Provider，状态留 VM（reqId 令牌丢弃过期响应）。
// 不返回 JSX、不持翻译串（MVVM 铁律）。

export interface CreateStrings {
  errorTitle: string;
}

export interface SaveStrings {
  savedTitle: string;
  savedMessage: string;
  errorTitle: string;
}

export interface ArchiveStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

export interface RemoveStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

export interface UseSpecsPageResult {
  projects: Project[];
  selectedPath: string;
  loading: boolean;
  busy: boolean;
  specs: Spec[];
  newTitle: string;
  setNewTitle: (v: string) => void;
  selectedId: string;
  content: string;
  contentDirty: boolean;
  contentLoading: boolean;
  setContent: (v: string) => void;
  select: (path: string, errorTitle: string) => Promise<void>;
  refresh: (errorTitle: string) => Promise<void>;
  create: (s: CreateStrings) => Promise<void>;
  openSpec: (spec: Spec, errorTitle: string) => Promise<void>;
  saveContent: (s: SaveStrings) => Promise<void>;
  activate: (spec: Spec, errorTitle: string) => Promise<void>;
  archive: (spec: Spec, s: ArchiveStrings) => Promise<void>;
  remove: (spec: Spec, s: RemoveStrings) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export const useSpecsPage = (): UseSpecsPageResult => {
  const { projects } = useProjects();
  const [selectedPath, setSelectedPath] = useState('');
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [specs, setSpecs] = useState<Spec[]>([]);
  const [newTitle, setNewTitle] = useState('');
  const [selectedId, setSelectedId] = useState('');
  const [content, setContentState] = useState('');
  const [contentDirty, setContentDirty] = useState(false);
  const [contentLoading, setContentLoading] = useState(false);

  const reqIdRef = useRef(0);
  const contentReqRef = useRef(0);

  const clearSelection = useCallback((): void => {
    contentReqRef.current++;
    setSelectedId('');
    setContentState('');
    setContentDirty(false);
    setContentLoading(false);
  }, []);

  const load = useCallback(async (path: string, errorTitle: string): Promise<void> => {
    const myReq = ++reqIdRef.current;
    setLoading(true);
    try {
      const rows = await SpecService.list(path, '');
      if (myReq !== reqIdRef.current) return;
      setSpecs(rows);
    } catch (e) {
      if (myReq === reqIdRef.current) setSpecs([]);
      await NativeDialogs.error(errorTitle, messageOf(e));
    } finally {
      if (myReq === reqIdRef.current) setLoading(false);
    }
  }, []);

  const select = useCallback(
    async (path: string, errorTitle: string): Promise<void> => {
      setSelectedPath(path);
      clearSelection();
      if (!path) {
        reqIdRef.current++;
        setSpecs([]);
        return;
      }
      await load(path, errorTitle);
    },
    [load, clearSelection],
  );

  const refresh = useCallback(
    async (errorTitle: string): Promise<void> => {
      if (!selectedPath) return;
      await load(selectedPath, errorTitle);
    },
    [selectedPath, load],
  );

  const setContent = useCallback((v: string): void => {
    setContentState(v);
    setContentDirty(true);
  }, []);

  // 打开某 spec 读其内容（令牌防过期）。供 openSpec 与 create（建后即打开）复用。
  const openSpec = useCallback(async (spec: Spec, errorTitle: string): Promise<void> => {
    const myReq = ++contentReqRef.current;
    setSelectedId(spec.id);
    setContentState('');
    setContentDirty(false);
    setContentLoading(true);
    try {
      const c = await SpecService.getContent(spec.projectPath, spec.id);
      if (myReq !== contentReqRef.current) return;
      setContentState(c);
    } catch (e) {
      if (myReq === contentReqRef.current) setContentState('');
      await NativeDialogs.error(errorTitle, messageOf(e));
    } finally {
      if (myReq === contentReqRef.current) setContentLoading(false);
    }
  }, []);

  // 新建 Spec：空标题 / busy 静默返回；成功后清输入、重拉、并打开新建的 spec。
  const create = useCallback(
    async (s: CreateStrings): Promise<void> => {
      const title = newTitle.trim();
      if (!selectedPath || busy || !title) return;
      setBusy(true);
      try {
        const created = await SpecService.create(selectedPath, title);
        setNewTitle('');
        await load(selectedPath, s.errorTitle);
        await openSpec(created, s.errorTitle);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedPath, busy, newTitle, load, openSpec],
  );

  const saveContent = useCallback(
    async (s: SaveStrings): Promise<void> => {
      if (!selectedPath || !selectedId || busy) return;
      setBusy(true);
      try {
        await SpecService.saveContent(selectedPath, selectedId, content);
        setContentDirty(false);
        await NativeDialogs.info(s.savedTitle, s.savedMessage);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedPath, selectedId, busy, content],
  );

  const activate = useCallback(
    async (spec: Spec, errorTitle: string): Promise<void> => {
      if (busy) return;
      setBusy(true);
      try {
        await SpecService.update(spec.id, { status: 'active' });
        await load(selectedPath, errorTitle);
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [busy, selectedPath, load],
  );

  const archive = useCallback(
    async (spec: Spec, s: ArchiveStrings): Promise<void> => {
      if (busy) return;
      const ok = await NativeDialogs.confirm({
        title: s.confirmTitle,
        message: s.confirmMessage,
        okLabel: s.okLabel,
        cancelLabel: s.cancelLabel,
      });
      if (!ok) return;
      setBusy(true);
      try {
        await SpecService.update(spec.id, { status: 'archived' });
        // 归档移动了文件；若正打开它则刷新内容（路径变了）。
        await load(selectedPath, s.errorTitle);
        if (selectedId === spec.id) clearSelection();
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [busy, selectedPath, selectedId, load, clearSelection],
  );

  const remove = useCallback(
    async (spec: Spec, s: RemoveStrings): Promise<void> => {
      if (busy) return;
      const ok = await NativeDialogs.confirm({
        title: s.confirmTitle,
        message: s.confirmMessage,
        okLabel: s.okLabel,
        cancelLabel: s.cancelLabel,
      });
      if (!ok) return;
      setBusy(true);
      try {
        await SpecService.remove(selectedPath, spec.id);
        if (selectedId === spec.id) clearSelection();
        await load(selectedPath, s.errorTitle);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [busy, selectedPath, selectedId, load, clearSelection],
  );

  return {
    projects,
    selectedPath,
    loading,
    busy,
    specs,
    newTitle,
    setNewTitle,
    selectedId,
    content,
    contentDirty,
    contentLoading,
    setContent,
    select,
    refresh,
    create,
    openSpec,
    saveContent,
    activate,
    archive,
    remove,
  };
};
