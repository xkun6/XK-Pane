import { useCallback, useEffect, useRef, useState } from 'react';
import { GitService, type GitFileStatus } from '@/services';
import { useFileChangeEvent } from '@/shared/hooks/useFileChangeEvent';

// 文件树 Git 状态装饰（借鉴 VS Code IDecorationsProvider）：按 rootPath 拉一次 StatusFiles + RepoRoot，
// 把仓库相对路径还原成绝对路径建 map，文件行据此上色 + 角标；变更文件的祖先目录滚动标记（folder roll-up）。
// 跟随 file:changed 防抖重拉（大仓库不在渲染期同步调用）。非 Git 仓库 → 空装饰（catch 静默）。

export type GitDecoKind = 'modified' | 'added' | 'deleted' | 'conflict';
export interface GitDecoration {
  letter: string; // M/A/D/U/R/C
  kind: GitDecoKind; // 驱动 foundation.status 配色
}

export interface GitDecorations {
  // 绝对路径（归一后）→ 装饰；文件行查表。
  fileDeco: (absPath: string) => GitDecoration | undefined;
  // 该目录下是否含变更（folder roll-up，折叠目录也能看出内部有改动）。
  dirChanged: (absPath: string) => boolean;
}

// 归一仅用于比较：统一分隔符、去尾斜杠、小写（Windows 大小写不敏感）。不改实际路径（D7）。
const norm = (p: string): string => p.replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
const parentOf = (key: string): string => key.replace(/\/[^/]*$/, '');

// 由 git XY 状态码 + flags 推一个展示字母与配色类别（index 优先于 worktree）。
const decoOf = (f: GitFileStatus): GitDecoration => {
  if (f.conflicted) return { letter: 'C', kind: 'conflict' };
  if (f.untracked) return { letter: 'U', kind: 'added' };
  const x = f.index !== ' ' ? f.index : f.worktree;
  switch (x) {
    case 'A':
      return { letter: 'A', kind: 'added' };
    case 'D':
      return { letter: 'D', kind: 'deleted' };
    case 'R':
      return { letter: 'R', kind: 'modified' };
    default:
      return { letter: 'M', kind: 'modified' };
  }
};

export const useGitDecorations = (rootPath: string): GitDecorations => {
  const [fileMap, setFileMap] = useState<Map<string, GitDecoration>>(() => new Map());
  const [dirSet, setDirSet] = useState<Set<string>>(() => new Set());

  const refresh = useCallback(async () => {
    if (!rootPath) {
      setFileMap(new Map());
      setDirSet(new Set());
      return;
    }
    try {
      const [root, files] = await Promise.all([
        GitService.repoRoot(rootPath),
        GitService.statusFiles(rootPath),
      ]);
      const cleanRoot = root.replace(/[\\/]+$/, '');
      const rootKey = norm(rootPath);
      const m = new Map<string, GitDecoration>();
      const dirs = new Set<string>();
      for (const f of files) {
        const key = norm(`${cleanRoot}/${f.path.replace(/^[\\/]+/, '')}`);
        m.set(key, decoOf(f));
        // 祖先目录滚动标记：从文件父目录向上到 rootPath（不含）逐级标记。
        let d = parentOf(key);
        while (d.length > rootKey.length && d.startsWith(rootKey)) {
          dirs.add(d);
          d = parentOf(d);
        }
      }
      setFileMap(m);
      setDirSet(dirs);
    } catch {
      // 非 Git 仓库 / Web 端：无装饰。
      setFileMap(new Map());
      setDirSet(new Set());
    }
  }, [rootPath]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // 跟随磁盘变更防抖重拉（大仓库 StatusFiles 有开销，合并抖动）。
  const timerRef = useRef<number | undefined>(undefined);
  useFileChangeEvent(
    useCallback(() => {
      if (timerRef.current !== undefined) window.clearTimeout(timerRef.current);
      timerRef.current = window.setTimeout(() => void refresh(), 400);
    }, [refresh]),
  );
  useEffect(() => () => window.clearTimeout(timerRef.current), []);

  const fileDeco = useCallback((absPath: string) => fileMap.get(norm(absPath)), [fileMap]);
  const dirChanged = useCallback((absPath: string) => dirSet.has(norm(absPath)), [dirSet]);
  return { fileDeco, dirChanged };
};
