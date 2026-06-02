import { useCallback, useEffect, useState } from 'react';
import { FileService, type FileEntry } from '@/services';

type ChildrenMap = Record<string, FileEntry[]>;

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
}

// 目录树状态：懒加载（展开某目录才拉它的直接子项并缓存）。rootPath 变化时整树重置。
export const useFileSystemTree = (rootPath: string): UseFileSystemTreeResult => {
  const [childrenByPath, setChildrenByPath] = useState<ChildrenMap>({});
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [loading, setLoading] = useState<Set<string>>(() => new Set());
  const [errored, setErrored] = useState<Set<string>>(() => new Set());

  const loadChildren = useCallback(async (path: string) => {
    setLoading((s) => new Set(s).add(path));
    setErrored((s) => {
      const n = new Set(s);
      n.delete(path);
      return n;
    });
    try {
      const entries = await FileService.listDir(path);
      setChildrenByPath((m) => ({ ...m, [path]: entries }));
    } catch {
      setErrored((s) => new Set(s).add(path));
    } finally {
      setLoading((s) => {
        const n = new Set(s);
        n.delete(path);
        return n;
      });
    }
  }, []);

  // rootPath 变化：清空缓存与展开态，重新加载根目录直接子项。
  useEffect(() => {
    setChildrenByPath({});
    setExpanded(new Set());
    setErrored(new Set());
    if (rootPath) void loadChildren(rootPath);
  }, [rootPath, loadChildren]);

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
      void loadChildren(path);
    },
    [loadChildren],
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
  };
};
