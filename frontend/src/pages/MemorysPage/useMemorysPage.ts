import { useCallback, useEffect, useRef, useState } from 'react';
import { MemoryService, NativeDialogs, type Memory, type MemoryStats } from '@/services';

// MemorysPage ViewModel：记忆是 app 级数据（按 scope 分级），故不依赖项目选择。管理过滤/搜索、
// 列表、统计、新建草稿、选中条目编辑。不返回 JSX、不持翻译串（MVVM 铁律）。
// 列表数据源始终走服务端 Search（含 LIKE 全文 + 过滤），mutation 后重拉。

const LIST_LIMIT = 200;

// 新建草稿（tags 以逗号分隔串编辑；scopeRef 仅非 global 用）。
export interface MemoryDraft {
  title: string;
  content: string;
  scope: string;
  scopeRef: string;
  category: string;
  importance: number;
  tags: string;
}

// 选中条目的可编辑字段（对齐 UpdateMemoryInput，无 scope）。
export interface MemoryEdit {
  title: string;
  content: string;
  category: string;
  importance: number;
  tags: string;
}

const EMPTY_DRAFT: MemoryDraft = {
  title: '',
  content: '',
  scope: 'global',
  scopeRef: '',
  category: 'fact',
  importance: 3,
  tags: '',
};

export interface CreateStrings {
  errorTitle: string;
  scopeRefRequired: string;
}

export interface SaveStrings {
  savedTitle: string;
  savedMessage: string;
  errorTitle: string;
}

export interface RemoveStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

export interface UseMemorysPageResult {
  items: Memory[];
  total: number;
  loading: boolean;
  busy: boolean;
  stats: MemoryStats | null;
  search: string;
  setSearch: (v: string) => void;
  scopeFilter: string;
  setScopeFilter: (v: string) => void;
  categoryFilter: string;
  setCategoryFilter: (v: string) => void;
  draft: MemoryDraft;
  patchDraft: (p: Partial<MemoryDraft>) => void;
  selectedId: string;
  edit: MemoryEdit;
  editDirty: boolean;
  patchEdit: (p: Partial<MemoryEdit>) => void;
  refresh: (errorTitle: string) => Promise<void>;
  create: (s: CreateStrings) => Promise<void>;
  openMemory: (m: Memory) => void;
  saveEdit: (s: SaveStrings) => Promise<void>;
  remove: (m: Memory, s: RemoveStrings) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

function parseTags(s: string): string[] {
  return s
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean);
}

export const useMemorysPage = (): UseMemorysPageResult => {
  const [items, setItems] = useState<Memory[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [stats, setStats] = useState<MemoryStats | null>(null);
  const [search, setSearch] = useState('');
  const [scopeFilter, setScopeFilter] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');
  const [draft, setDraft] = useState<MemoryDraft>(EMPTY_DRAFT);
  const [selectedId, setSelectedId] = useState('');
  const [edit, setEdit] = useState<MemoryEdit>({ title: '', content: '', category: 'fact', importance: 3, tags: '' });
  const [editDirty, setEditDirty] = useState(false);

  const reqIdRef = useRef(0);

  const load = useCallback(
    async (errorTitle: string, opts: { search: string; scope: string; category: string }): Promise<void> => {
      const myReq = ++reqIdRef.current;
      setLoading(true);
      try {
        const [res, st] = await Promise.all([
          MemoryService.search({
            search: opts.search.trim() || undefined,
            scope: opts.scope || undefined,
            category: opts.category || undefined,
            limit: LIST_LIMIT,
          }),
          MemoryService.stats(),
        ]);
        if (myReq !== reqIdRef.current) return;
        setItems(res.items);
        setTotal(res.total);
        setStats(st);
      } catch (e) {
        if (myReq === reqIdRef.current) {
          setItems([]);
          setTotal(0);
        }
        await NativeDialogs.error(errorTitle, messageOf(e));
      } finally {
        if (myReq === reqIdRef.current) setLoading(false);
      }
    },
    [],
  );

  // 过滤/搜索变化 → 防抖重拉（200ms 合并搜索连打）。
  useEffect(() => {
    const handle = setTimeout(() => {
      void load('', { search, scope: scopeFilter, category: categoryFilter });
    }, 200);
    return () => clearTimeout(handle);
  }, [search, scopeFilter, categoryFilter, load]);

  const refresh = useCallback(
    async (errorTitle: string): Promise<void> => {
      await load(errorTitle, { search, scope: scopeFilter, category: categoryFilter });
    },
    [load, search, scopeFilter, categoryFilter],
  );

  const patchDraft = useCallback((p: Partial<MemoryDraft>): void => {
    setDraft((prev) => ({ ...prev, ...p }));
  }, []);

  const patchEdit = useCallback((p: Partial<MemoryEdit>): void => {
    setEdit((prev) => ({ ...prev, ...p }));
    setEditDirty(true);
  }, []);

  // 新建记忆：标题 / 内容空，或非 global 缺 scopeRef → 提示返回；成功后重置草稿 + 重拉。
  const create = useCallback(
    async (s: CreateStrings): Promise<void> => {
      if (busy) return;
      const title = draft.title.trim();
      const content = draft.content.trim();
      if (!title || !content) return;
      const ref = draft.scopeRef.trim();
      if (draft.scope !== 'global' && !ref) {
        await NativeDialogs.error(s.errorTitle, s.scopeRefRequired);
        return;
      }
      setBusy(true);
      try {
        await MemoryService.store({
          title,
          content,
          scope: draft.scope,
          category: draft.category,
          importance: draft.importance,
          tags: parseTags(draft.tags),
          ...(draft.scope === 'workspace' ? { workspaceName: ref } : {}),
          ...(draft.scope === 'project' ? { projectPath: ref } : {}),
          ...(draft.scope === 'session' ? { sessionId: ref } : {}),
        });
        setDraft(EMPTY_DRAFT);
        await load(s.errorTitle, { search, scope: scopeFilter, category: categoryFilter });
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [busy, draft, load, search, scopeFilter, categoryFilter],
  );

  const openMemory = useCallback((m: Memory): void => {
    setSelectedId(m.id);
    setEdit({
      title: m.title,
      content: m.content,
      category: m.category,
      importance: m.importance,
      tags: m.tags.join(', '),
    });
    setEditDirty(false);
  }, []);

  const saveEdit = useCallback(
    async (s: SaveStrings): Promise<void> => {
      if (!selectedId || busy) return;
      setBusy(true);
      try {
        await MemoryService.update(selectedId, {
          title: edit.title,
          content: edit.content,
          category: edit.category,
          importance: edit.importance,
          tags: parseTags(edit.tags),
        });
        setEditDirty(false);
        await NativeDialogs.info(s.savedTitle, s.savedMessage);
        await load(s.errorTitle, { search, scope: scopeFilter, category: categoryFilter });
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedId, busy, edit, load, search, scopeFilter, categoryFilter],
  );

  const remove = useCallback(
    async (m: Memory, s: RemoveStrings): Promise<void> => {
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
        await MemoryService.remove(m.id);
        if (selectedId === m.id) setSelectedId('');
        await load(s.errorTitle, { search, scope: scopeFilter, category: categoryFilter });
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [busy, selectedId, load, search, scopeFilter, categoryFilter],
  );

  return {
    items,
    total,
    loading,
    busy,
    stats,
    search,
    setSearch,
    scopeFilter,
    setScopeFilter,
    categoryFilter,
    setCategoryFilter,
    draft,
    patchDraft,
    selectedId,
    edit,
    editDirty,
    patchEdit,
    refresh,
    create,
    openMemory,
    saveEdit,
    remove,
  };
};
