import { useCallback, useMemo, useRef, useState } from 'react';
import {
  GitService,
  NativeDialogs,
  type GitBranch,
  type GitCommit,
  type GitFileStatus,
  type GitStatus,
  type GitWorktree,
  type DiffResult,
  type Project,
} from '@/services';
import { useProjects } from '@/project';

// GitPage ViewModel：消费全局 ProjectProvider 选项目 + 按 path 查询 Git 状态 / 工具条 /
// VSCode 式源代码管理（变更文件分组 + per-file diff + stage/unstage/discard/commit）。
//
// 为何不建全局 GitProvider：Git 状态是**按 path 即查**的视图态（非跨页面共享的全局登记表）。
// 故状态留在本 VM；若将来侧边栏需 per-project Git 角标等多消费方，再提升为 Provider。
//
// 不返回 JSX、不持翻译串（错误/确认标题由 View 经 t() 传入，MVVM 铁律）。

const LOG_LIMIT = 20;

// 工具条操作的文案束（由 View 经 t() 填好后传入；VM 不持翻译串）。
export interface OpStrings {
  successTitle: string;
  successMessage: string; // void 返回的操作用此；stash/pop 优先用 git 返回文本
  errorTitle: string;
}

// push 额外需要确认对话框文案（推送是对外不可逆操作）。
export interface PushStrings extends OpStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
}

// 新建 worktree：需目录选择器标题 + 成功/失败文案（名称取 VM 的 newWorktreeName）。
export interface AddWorktreeStrings {
  pickerTitle: string;
  successTitle: string;
  successMessage: string;
  errorTitle: string;
}

// 删除 worktree：需确认文案 + 成功/失败文案。
export interface RemoveWorktreeStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  successTitle: string;
  successMessage: string;
  errorTitle: string;
}

// 丢弃改动：破坏性操作，需确认文案 + 失败文案（成功静默，列表刷新即反馈）。
export interface DiscardStrings {
  confirmTitle: string;
  confirmMessage: string; // 含 {{path}} / {{count}} 由 View 预先插值
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

// 当前查看的 diff 选中项（哪个文件 + 看暂存区侧还是工作树侧）。
export interface SelectedDiff {
  file: string;
  staged: boolean;
}

export interface UseGitPageResult {
  projects: Project[];
  selectedPath: string;
  loading: boolean;
  busy: boolean;
  status: GitStatus | null;
  branches: GitBranch[];
  commits: GitCommit[];
  worktrees: GitWorktree[];
  newWorktreeName: string;
  setNewWorktreeName: (v: string) => void;
  // —— SCM ——
  stagedFiles: GitFileStatus[];
  changedFiles: GitFileStatus[]; // 未暂存 + 未跟踪
  hasStaged: boolean;
  commitMessage: string;
  setCommitMessage: (v: string) => void;
  selectedDiff: SelectedDiff | null;
  diff: DiffResult | null;
  diffLoading: boolean;
  select: (path: string, errorTitle: string) => Promise<void>;
  refresh: (errorTitle: string) => Promise<void>;
  fetch: (s: OpStrings) => Promise<void>;
  pull: (s: OpStrings) => Promise<void>;
  push: (s: PushStrings) => Promise<void>;
  stash: (s: OpStrings) => Promise<void>;
  stashPop: (s: OpStrings) => Promise<void>;
  addWorktree: (s: AddWorktreeStrings) => Promise<void>;
  removeWorktree: (wt: GitWorktree, s: RemoveWorktreeStrings) => Promise<void>;
  openDiff: (file: string, staged: boolean, errorTitle: string) => Promise<void>;
  closeDiff: () => void;
  stageFile: (file: string, errorTitle: string) => Promise<void>;
  unstageFile: (file: string, errorTitle: string) => Promise<void>;
  discardFile: (file: string, s: DiscardStrings) => Promise<void>;
  stageAll: (errorTitle: string) => Promise<void>;
  unstageAll: (errorTitle: string) => Promise<void>;
  discardAll: (s: DiscardStrings) => Promise<void>;
  commit: (errorTitle: string) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export const useGitPage = (): UseGitPageResult => {
  const { projects } = useProjects();
  const [selectedPath, setSelectedPath] = useState('');
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<GitStatus | null>(null);
  const [branches, setBranches] = useState<GitBranch[]>([]);
  const [commits, setCommits] = useState<GitCommit[]>([]);
  const [worktrees, setWorktrees] = useState<GitWorktree[]>([]);
  const [newWorktreeName, setNewWorktreeName] = useState('');
  const [files, setFiles] = useState<GitFileStatus[]>([]);
  const [commitMessage, setCommitMessage] = useState('');
  const [selectedDiff, setSelectedDiff] = useState<SelectedDiff | null>(null);
  const [diff, setDiff] = useState<DiffResult | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);

  // 请求令牌：快速切换项目 / 文件时丢弃过期响应，只采纳最新一次。
  const reqIdRef = useRef(0);
  const diffReqRef = useRef(0);
  // selectedDiff 的镜像 ref：供 SCM 操作后重拉当前 diff（避免把 selectedDiff 塞进 callback deps）。
  const selectedDiffRef = useRef<SelectedDiff | null>(null);
  selectedDiffRef.current = selectedDiff;

  const stagedFiles = useMemo(() => files.filter((f) => f.staged), [files]);
  const changedFiles = useMemo(() => files.filter((f) => f.unstaged), [files]);

  const clearDetails = useCallback((): void => {
    setStatus(null);
    setBranches([]);
    setCommits([]);
    setWorktrees([]);
    setFiles([]);
    setSelectedDiff(null);
    setDiff(null);
  }, []);

  const load = useCallback(
    async (path: string, errorTitle: string): Promise<void> => {
      const myReq = ++reqIdRef.current;
      setLoading(true);
      try {
        const st = await GitService.getStatus(path);
        if (myReq !== reqIdRef.current) return; // 已被更新的请求取代
        setStatus(st);
        if (st.isRepo) {
          const [brs, logs, wts, fls] = await Promise.all([
            GitService.branches(path),
            GitService.log(path, LOG_LIMIT),
            GitService.worktrees(path),
            GitService.statusFiles(path),
          ]);
          if (myReq !== reqIdRef.current) return;
          setBranches(brs);
          setCommits(logs);
          setWorktrees(wts);
          setFiles(fls);
        } else {
          setBranches([]);
          setCommits([]);
          setWorktrees([]);
          setFiles([]);
        }
      } catch (e) {
        if (myReq === reqIdRef.current) clearDetails();
        await NativeDialogs.error(errorTitle, messageOf(e));
      } finally {
        if (myReq === reqIdRef.current) setLoading(false);
      }
    },
    [clearDetails],
  );

  const select = useCallback(
    async (path: string, errorTitle: string): Promise<void> => {
      setSelectedPath(path);
      setSelectedDiff(null);
      setDiff(null);
      setCommitMessage('');
      if (!path) {
        reqIdRef.current++; // 取消在途请求
        clearDetails();
        return;
      }
      await load(path, errorTitle);
    },
    [load, clearDetails],
  );

  const refresh = useCallback(
    async (errorTitle: string): Promise<void> => {
      if (!selectedPath) return;
      await load(selectedPath, errorTitle);
    },
    [selectedPath, load],
  );

  // —— diff 查看 ——

  const fetchDiff = useCallback(
    async (path: string, file: string, staged: boolean, errorTitle: string): Promise<void> => {
      const myReq = ++diffReqRef.current;
      setDiffLoading(true);
      try {
        const d = await GitService.diffFile(path, file, staged);
        if (myReq !== diffReqRef.current) return;
        setDiff(d);
      } catch (e) {
        if (myReq === diffReqRef.current) setDiff(null);
        await NativeDialogs.error(errorTitle, messageOf(e));
      } finally {
        if (myReq === diffReqRef.current) setDiffLoading(false);
      }
    },
    [],
  );

  const openDiff = useCallback(
    async (file: string, staged: boolean, errorTitle: string): Promise<void> => {
      if (!selectedPath) return;
      setSelectedDiff({ file, staged });
      await fetchDiff(selectedPath, file, staged, errorTitle);
    },
    [selectedPath, fetchDiff],
  );

  const closeDiff = useCallback((): void => {
    diffReqRef.current++; // 取消在途 diff 请求
    setSelectedDiff(null);
    setDiff(null);
  }, []);

  // —— 工具条操作（fetch/pull/push/stash/stashPop）——

  const runOp = useCallback(
    async (fn: (path: string) => Promise<string | void>, s: OpStrings, showResult: boolean): Promise<void> => {
      if (!selectedPath || busy) return;
      setBusy(true);
      try {
        const res = await fn(selectedPath);
        const msg = showResult && typeof res === 'string' && res.trim() ? res.trim() : s.successMessage;
        await NativeDialogs.info(s.successTitle, msg);
        await load(selectedPath, s.errorTitle);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedPath, busy, load],
  );

  const fetch = useCallback((s: OpStrings) => runOp(GitService.fetch, s, false), [runOp]);
  const pull = useCallback((s: OpStrings) => runOp(GitService.pull, s, false), [runOp]);
  const stash = useCallback((s: OpStrings) => runOp(GitService.stash, s, true), [runOp]);
  const stashPop = useCallback((s: OpStrings) => runOp(GitService.stashPop, s, true), [runOp]);

  const push = useCallback(
    async (s: PushStrings): Promise<void> => {
      if (!selectedPath || busy) return;
      const ok = await NativeDialogs.confirm({
        title: s.confirmTitle,
        message: s.confirmMessage,
        okLabel: s.okLabel,
        cancelLabel: s.cancelLabel,
      });
      if (!ok) return;
      await runOp(GitService.push, s, false);
    },
    [selectedPath, busy, runOp],
  );

  // —— SCM 变更操作：成功静默（列表刷新即反馈，对齐 VSCode），仅失败弹错。操作后重拉状态，
  // 并刷新当前打开的 diff（其内容可能因 stage/unstage 改变）。——

  const runScm = useCallback(
    async (fn: () => Promise<void>, errorTitle: string): Promise<void> => {
      if (!selectedPath || busy) return;
      setBusy(true);
      try {
        await fn();
        await load(selectedPath, errorTitle);
        const sel = selectedDiffRef.current;
        if (sel) await fetchDiff(selectedPath, sel.file, sel.staged, errorTitle);
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedPath, busy, load, fetchDiff],
  );

  const stageFile = useCallback(
    (file: string, errorTitle: string) => runScm(() => GitService.stage(selectedPath, [file]), errorTitle),
    [runScm, selectedPath],
  );
  const unstageFile = useCallback(
    (file: string, errorTitle: string) => runScm(() => GitService.unstage(selectedPath, [file]), errorTitle),
    [runScm, selectedPath],
  );
  const stageAll = useCallback(
    (errorTitle: string) => runScm(() => GitService.stage(selectedPath, changedFiles.map((f) => f.path)), errorTitle),
    [runScm, selectedPath, changedFiles],
  );
  const unstageAll = useCallback(
    (errorTitle: string) => runScm(() => GitService.unstage(selectedPath, stagedFiles.map((f) => f.path)), errorTitle),
    [runScm, selectedPath, stagedFiles],
  );

  const discardWith = useCallback(
    async (paths: string[], s: DiscardStrings): Promise<void> => {
      if (!selectedPath || busy || paths.length === 0) return;
      const ok = await NativeDialogs.confirm({
        title: s.confirmTitle,
        message: s.confirmMessage,
        okLabel: s.okLabel,
        cancelLabel: s.cancelLabel,
      });
      if (!ok) return;
      await runScm(() => GitService.discard(selectedPath, paths), s.errorTitle);
    },
    [selectedPath, busy, runScm],
  );

  const discardFile = useCallback((file: string, s: DiscardStrings) => discardWith([file], s), [discardWith]);
  const discardAll = useCallback(
    (s: DiscardStrings) => discardWith(changedFiles.map((f) => f.path), s),
    [discardWith, changedFiles],
  );

  const commit = useCallback(
    async (errorTitle: string): Promise<void> => {
      const msg = commitMessage.trim();
      if (!selectedPath || busy || !msg) return;
      setBusy(true);
      try {
        await GitService.commit(selectedPath, msg);
        setCommitMessage('');
        setSelectedDiff(null);
        setDiff(null);
        await load(selectedPath, errorTitle);
      } catch (e) {
        await NativeDialogs.error(errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedPath, busy, commitMessage, load],
  );

  // 新建 worktree：名取 newWorktreeName（同时作子目录名与新分支名）→ 选父目录 →
  // 后端 join + 名校验 + 建树 → 成功后清空输入并重拉。空名 / 取消选择器 → 静默返回。
  const addWorktree = useCallback(
    async (s: AddWorktreeStrings): Promise<void> => {
      const name = newWorktreeName.trim();
      if (!selectedPath || busy || !name) return;
      const parentDir = await NativeDialogs.openDirectory({ title: s.pickerTitle });
      if (!parentDir) return;
      setBusy(true);
      try {
        await GitService.worktreeAddUnder(selectedPath, parentDir, name, name);
        setNewWorktreeName('');
        await NativeDialogs.info(s.successTitle, s.successMessage);
        await load(selectedPath, s.errorTitle);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedPath, busy, newWorktreeName, load],
  );

  const removeWorktree = useCallback(
    async (wt: GitWorktree, s: RemoveWorktreeStrings): Promise<void> => {
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
        await GitService.worktreeRemove(selectedPath, wt.path);
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
    status,
    branches,
    commits,
    worktrees,
    newWorktreeName,
    setNewWorktreeName,
    stagedFiles,
    changedFiles,
    hasStaged: stagedFiles.length > 0,
    commitMessage,
    setCommitMessage,
    selectedDiff,
    diff,
    diffLoading,
    select,
    refresh,
    fetch,
    pull,
    push,
    stash,
    stashPop,
    addWorktree,
    removeWorktree,
    openDiff,
    closeDiff,
    stageFile,
    unstageFile,
    discardFile,
    stageAll,
    unstageAll,
    discardAll,
    commit,
  };
};
