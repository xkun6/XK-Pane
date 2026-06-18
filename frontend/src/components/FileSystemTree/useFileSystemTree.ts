import { useCallback, useEffect, useRef, useState } from 'react';
import { FileService, FilewatchService, type FileEntry } from '@/services';
import { useFileChangeEvent } from '@/shared/hooks/useFileChangeEvent';

type ChildrenMap = Record<string, FileEntry[]>;

// 一次文件树写操作的结果通知（供上层同步编辑器选中态等）。
export interface FileTreeMutation {
  kind: 'create' | 'rename' | 'duplicate' | 'trash' | 'move';
  parentDir: string;
  oldPath?: string;
  newPath?: string;
}

// 剪贴板：复制 / 剪切的待粘贴条目。空表示无可粘贴内容。
export interface FileClipboard {
  entry: FileEntry;
  mode: 'copy' | 'cut';
}

export interface UseFileSystemTreeResult {
  rootEntries: FileEntry[];
  childrenByPath: ChildrenMap;
  expanded: Set<string>;
  loading: Set<string>;
  errored: Set<string>;
  rootLoading: boolean;
  rootErrored: boolean;
  toggle: (path: string) => void;
  reload: (path: string) => void;
  // 写操作：成功后自动刷新受影响目录并经 onAfterMutate 通知。失败抛错由调用方提示。
  createFile: (dir: string, name: string) => Promise<void>;
  createDir: (dir: string, name: string) => Promise<void>;
  renameEntry: (entry: FileEntry, newName: string) => Promise<void>;
  duplicateEntry: (entry: FileEntry) => Promise<void>;
  trashEntry: (entry: FileEntry) => Promise<void>;
  // 移动 entry 到 dstDir（拖拽 / 剪切粘贴共用）：刷新源与目标目录，移动的若是打开文件经 mutation 跟随。
  moveEntryInto: (entry: FileEntry, dstDir: string) => Promise<void>;
  // 剪贴板：复制 / 剪切登记 + 粘贴到目标目录。
  clipboard: FileClipboard | null;
  copyEntry: (entry: FileEntry) => void;
  cutEntry: (entry: FileEntry) => void;
  pasteInto: (dir: string) => Promise<void>;
}

const dirOf = (p: string): string => p.replace(/[\\/][^\\/]*$/, '');

// 目录树状态：懒加载（展开某目录才拉它的直接子项并缓存）。rootPath 变化时整树重置。
export const useFileSystemTree = (
  rootPath: string,
  onAfterMutate?: (m: FileTreeMutation) => void,
): UseFileSystemTreeResult => {
  const [childrenByPath, setChildrenByPath] = useState<ChildrenMap>({});
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [loading, setLoading] = useState<Set<string>>(() => new Set());
  const [errored, setErrored] = useState<Set<string>>(() => new Set());
  const [clipboard, setClipboard] = useState<FileClipboard | null>(null);

  // silent=true：刷新已加载目录时不动 loading/errored 态（避免「加载中」闪屏），
  // 失败保留旧数据。供自动刷新(file:changed)与写操作后的刷新使用。
  const loadChildren = useCallback(async (path: string, silent = false) => {
    if (!silent) {
      setLoading((s) => new Set(s).add(path));
      setErrored((s) => {
        const n = new Set(s);
        n.delete(path);
        return n;
      });
    }
    try {
      const entries = await FileService.listDir(path);
      setChildrenByPath((m) => ({ ...m, [path]: entries }));
      if (silent) {
        setErrored((s) => {
          if (!s.has(path)) return s;
          const n = new Set(s);
          n.delete(path);
          return n;
        });
      }
    } catch {
      if (!silent) setErrored((s) => new Set(s).add(path));
      // silent 失败：保留旧数据，不显错（外部短暂状态不该让树闪烁）
    } finally {
      if (!silent) {
        setLoading((s) => {
          const n = new Set(s);
          n.delete(path);
          return n;
        });
      }
    }
  }, []);

  // rootPath 变化：清空缓存与展开态，重新加载根目录直接子项，并切换磁盘变更监听到新根。
  useEffect(() => {
    setChildrenByPath({});
    setExpanded(new Set());
    setErrored(new Set());
    if (rootPath) {
      void loadChildren(rootPath);
      void FilewatchService.watch(rootPath);
    }
    return () => {
      void FilewatchService.unwatch();
    };
  }, [rootPath, loadChildren]);

  // 外部磁盘变更（agent 写文件等）：刷新「已加载」且命中变更目录集的目录（分隔符/大小写归一比较）。
  const childrenRef = useRef(childrenByPath);
  childrenRef.current = childrenByPath;
  useFileChangeEvent(
    useCallback(
      (dirs: string[]) => {
        const norm = (p: string) => p.replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
        const changed = new Set(dirs.map(norm));
        for (const key of Object.keys(childrenRef.current)) {
          if (changed.has(norm(key))) void loadChildren(key, true); // 静默刷新，防闪屏
        }
      },
      [loadChildren],
    ),
  );

  const toggle = useCallback(
    (path: string) => {
      const willExpand = !expanded.has(path);
      setExpanded((s) => {
        const n = new Set(s);
        if (willExpand) n.add(path);
        else n.delete(path);
        return n;
      });
      if (willExpand && !(path in childrenByPath)) void loadChildren(path);
    },
    [expanded, childrenByPath, loadChildren],
  );

  const reload = useCallback(
    (path: string) => {
      void loadChildren(path, true);
    },
    [loadChildren],
  );

  // 在 dir 下新建后：展开并刷新该目录，使新条目可见。
  const createFile = useCallback(
    async (dir: string, name: string) => {
      await FileService.createFile(dir, name);
      setExpanded((s) => new Set(s).add(dir));
      await loadChildren(dir, true);
      onAfterMutate?.({ kind: 'create', parentDir: dir });
    },
    [loadChildren, onAfterMutate],
  );

  const createDir = useCallback(
    async (dir: string, name: string) => {
      await FileService.createDir(dir, name);
      setExpanded((s) => new Set(s).add(dir));
      await loadChildren(dir, true);
      onAfterMutate?.({ kind: 'create', parentDir: dir });
    },
    [loadChildren, onAfterMutate],
  );

  const renameEntry = useCallback(
    async (entry: FileEntry, newName: string) => {
      const newPath = await FileService.rename(entry.path, newName);
      const parent = dirOf(entry.path);
      await loadChildren(parent, true);
      onAfterMutate?.({ kind: 'rename', parentDir: parent, oldPath: entry.path, newPath });
    },
    [loadChildren, onAfterMutate],
  );

  const duplicateEntry = useCallback(
    async (entry: FileEntry) => {
      await FileService.duplicate(entry.path);
      const parent = dirOf(entry.path);
      await loadChildren(parent, true);
      onAfterMutate?.({ kind: 'duplicate', parentDir: parent });
    },
    [loadChildren, onAfterMutate],
  );

  const trashEntry = useCallback(
    async (entry: FileEntry) => {
      await FileService.trash(entry.path);
      const parent = dirOf(entry.path);
      await loadChildren(parent, true);
      onAfterMutate?.({ kind: 'trash', parentDir: parent, oldPath: entry.path });
    },
    [loadChildren, onAfterMutate],
  );

  // 移动 entry 到 dstDir：刷新目标目录与原父目录；后端对「移回原目录」为无操作（target===src）。
  const moveEntryInto = useCallback(
    async (entry: FileEntry, dstDir: string) => {
      const srcParent = dirOf(entry.path);
      const target = await FileService.move(entry.path, dstDir);
      setExpanded((s) => new Set(s).add(dstDir));
      await loadChildren(dstDir, true);
      if (srcParent !== dstDir) await loadChildren(srcParent, true);
      if (target !== entry.path) {
        onAfterMutate?.({ kind: 'move', parentDir: dstDir, oldPath: entry.path, newPath: target });
      }
    },
    [loadChildren, onAfterMutate],
  );

  const copyEntry = useCallback((entry: FileEntry) => setClipboard({ entry, mode: 'copy' }), []);
  const cutEntry = useCallback((entry: FileEntry) => setClipboard({ entry, mode: 'cut' }), []);

  // 粘贴到 dir：复制走 FileService.copy（源保留），剪切走 moveEntryInto 并清空剪贴板。
  const pasteInto = useCallback(
    async (dir: string) => {
      const clip = clipboard;
      if (!clip) return;
      if (clip.mode === 'cut') {
        await moveEntryInto(clip.entry, dir);
        setClipboard(null);
      } else {
        await FileService.copy(clip.entry.path, dir);
        setExpanded((s) => new Set(s).add(dir));
        await loadChildren(dir, true);
        onAfterMutate?.({ kind: 'create', parentDir: dir });
      }
    },
    [clipboard, moveEntryInto, loadChildren, onAfterMutate],
  );

  return {
    rootEntries: childrenByPath[rootPath] ?? [],
    childrenByPath,
    expanded,
    loading,
    errored,
    rootLoading: loading.has(rootPath),
    rootErrored: errored.has(rootPath),
    toggle,
    reload,
    createFile,
    createDir,
    renameEntry,
    duplicateEntry,
    trashEntry,
    moveEntryInto,
    clipboard,
    copyEntry,
    cutEntry,
    pasteInto,
  };
};
