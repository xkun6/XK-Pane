// GitService：Git 只读/工具门面的前端封装，包 @bindings/xkpane gitsvc。
//
// 后端 gitsvc 是 gitx 原语的薄无状态包装（按 path 操作）。本服务把生成的 binding
// 收口为干净 TS 类型。当前页面只消费**只读**子集（GetStatus / Branches / Log）；
// worktree / stash / 网络（fetch/pull/push/clone）是变更操作，待后续单元接入（含原生确认）。
import { Service as Binding } from '@bindings/xkpane/internal/services/gitsvc';
import type { StatusDTO as RawStatus } from '@bindings/xkpane/internal/services/gitsvc/models';
import type {
  BranchInfo as RawBranch,
  CommitInfo as RawCommit,
  WorktreeInfo as RawWorktree,
  FileStatus as RawFileStatus,
} from '@bindings/xkpane/internal/utils/gitx/models';
import type {
  DiffResult as RawDiff,
  DiffHunk as RawHunk,
  DiffLine as RawLine,
  InlineChange as RawInline,
} from '@bindings/xkpane/internal/localhistory/models';
// diff 复用 Local History 的 DiffResult 类型 + DiffView（后端 gitsvc.DiffFile 返回同一
// localhistory.DiffResult）。从具体模块直接 import 类型，避免经 @/services 桶导致自引循环。
import type {
  DiffResult,
  DiffHunk,
  DiffLine,
  InlineChange,
  DiffChangeType,
} from '../localhistory/LocalHistoryService';

// 仓库概要状态。isRepo=false 时 branch/isDirty 无意义（前端据此隐藏 Git 详情）。
export interface GitStatus {
  isRepo: boolean;
  branch: string;
  isDirty: boolean;
}

export interface GitBranch {
  name: string;
  isCurrent: boolean;
}

// 一条提交。when 是后端 time.Time 序列化的 RFC3339 字符串（生成类型为 any，运行时为字符串）。
export interface GitCommit {
  hash: string;
  shortHash: string;
  subject: string;
  author: string;
  when: string;
}

// 一条 worktree。path 由 git porcelain 输出（Windows 下为正斜杠）；isMain=主工作树（不可删）。
export interface GitWorktree {
  path: string;
  branch: string;
  commit: string;
  isMain: boolean;
}

// 一个变更文件（VSCode SCM）。index/worktree 为 git porcelain 的 XY 状态码（' ' = 该侧无变更）。
// 一个文件可同时 staged 且 unstaged（如先暂存修改后又改了）。
export interface GitFileStatus {
  path: string;
  origPath: string; // 重命名/复制来源（否则空）
  index: string; // 暂存区相对 HEAD：M/A/D/R/C/' '
  worktree: string; // 工作树相对暂存区：M/D/' '/'?'
  staged: boolean;
  unstaged: boolean;
  untracked: boolean;
  conflicted: boolean;
}

function toStatus(s: RawStatus): GitStatus {
  return {
    isRepo: s.isRepo ?? false,
    branch: s.branch ?? '',
    isDirty: s.isDirty ?? false,
  };
}

function toBranch(b: RawBranch): GitBranch {
  return {
    name: b.name ?? '',
    isCurrent: b.isCurrent ?? false,
  };
}

function toCommit(c: RawCommit): GitCommit {
  return {
    hash: c.hash ?? '',
    shortHash: c.shortHash ?? '',
    subject: c.subject ?? '',
    author: c.author ?? '',
    when: typeof c.when === 'string' ? c.when : '',
  };
}

function toWorktree(w: RawWorktree): GitWorktree {
  return {
    path: w.path ?? '',
    branch: w.branch ?? '',
    commit: w.commit ?? '',
    isMain: w.isMain ?? false,
  };
}

function toFileStatus(f: RawFileStatus): GitFileStatus {
  return {
    path: f.path ?? '',
    origPath: f.origPath ?? '',
    index: f.index ?? ' ',
    worktree: f.worktree ?? ' ',
    staged: f.staged ?? false,
    unstaged: f.unstaged ?? false,
    untracked: f.untracked ?? false,
    conflicted: f.conflicted ?? false,
  };
}

// —— diff 映射（后端 localhistory.DiffResult → 前端 DiffResult；与 LocalHistoryService 同形，
// 自包含以保持服务解耦）——
function toInline(c: RawInline): InlineChange {
  return {
    start: c.start ?? 0,
    end: c.end ?? 0,
    changeType: (c.changeType ?? 'equal') as DiffChangeType,
  };
}

function toLine(l: RawLine): DiffLine {
  const inlines = l.inlineChanges?.map(toInline);
  return {
    changeType: (l.changeType ?? 'equal') as DiffChangeType,
    content: l.content ?? '',
    oldLineNo: typeof l.oldLineNo === 'number' ? l.oldLineNo : null,
    newLineNo: typeof l.newLineNo === 'number' ? l.newLineNo : null,
    ...(inlines && inlines.length > 0 ? { inlineChanges: inlines } : {}),
  };
}

function toHunk(h: RawHunk): DiffHunk {
  return {
    oldStart: h.oldStart ?? 0,
    oldCount: h.oldCount ?? 0,
    newStart: h.newStart ?? 0,
    newCount: h.newCount ?? 0,
    lines: (h.lines ?? []).map(toLine),
  };
}

function toDiff(d: RawDiff): DiffResult {
  return {
    hunks: (d.hunks ?? []).map(toHunk),
    stats: {
      additions: d.stats?.additions ?? 0,
      deletions: d.stats?.deletions ?? 0,
      changes: d.stats?.changes ?? 0,
    },
    isBinary: d.isBinary ?? false,
    truncated: d.truncated ?? false,
  };
}

export const GitService = {
  // 廉价判定 path 是否一个 git 仓库（不 shell git）。
  async isRepo(path: string): Promise<boolean> {
    return Binding.IsRepo(path);
  },

  // 概要状态：非仓库友好降级为 {isRepo:false}（后端不报错）。
  async getStatus(path: string): Promise<GitStatus> {
    return toStatus(await Binding.GetStatus(path));
  },

  // 本地分支列表（标注当前分支）。非仓库会抛 ErrNotARepo（调用方应先判 isRepo）。
  async branches(path: string): Promise<GitBranch[]> {
    const rows = await Binding.Branches(path);
    return (rows ?? []).map(toBranch);
  },

  // 最近 limit 条提交（最新在前）；limit<=0 后端取默认 50，空仓回 []。
  async log(path: string, limit: number): Promise<GitCommit[]> {
    const rows = await Binding.Log(path, limit);
    return (rows ?? []).map(toCommit);
  },

  // —— 变更操作（git 工具条；失败抛 git 原始错误，调用方经 NativeDialogs 提示）——

  // 拉取所有远端引用（fetch --all）；只更新远端跟踪引用，不动工作树。
  async fetch(path: string): Promise<void> {
    await Binding.Fetch(path);
  },

  // 合并远端改动（git pull）；可能改动工作树。
  async pull(path: string): Promise<void> {
    await Binding.Pull(path);
  },

  // 推送当前分支（git push）；无上游分支会失败。
  async push(path: string): Promise<void> {
    await Binding.Push(path);
  },

  // 暂存工作区改动，返回 git 提示信息（如「Saved working directory ...」/「No local changes to save」）。
  async stash(path: string): Promise<string> {
    return Binding.Stash(path);
  },

  // 弹出最近一次 stash，返回 git 提示信息；无 stash 条目时失败。
  async stashPop(path: string): Promise<string> {
    return Binding.StashPop(path);
  },

  // —— worktree 管理 ——

  // 列出所有 worktree（含主工作树，首条为主）。非仓库会抛 ErrNotARepo。
  async worktrees(path: string): Promise<GitWorktree[]> {
    const rows = await Binding.Worktrees(path);
    return (rows ?? []).map(toWorktree);
  },

  // 在 parentDir 下以 name 为子目录名新建 worktree（后端做名校验 + OS 感知 join）；
  // newBranch 非空则创建并检出该新分支。返回创建出的绝对路径。
  async worktreeAddUnder(repoPath: string, parentDir: string, name: string, newBranch: string): Promise<string> {
    return Binding.WorktreeAddUnder(repoPath, parentDir, name, newBranch);
  },

  // 删除 worktreePath 处的 worktree（恒带 --force）。
  async worktreeRemove(repoPath: string, worktreePath: string): Promise<void> {
    await Binding.WorktreeRemove(repoPath, worktreePath);
  },

  // —— VSCode 源代码管理（SCM）——

  // 结构化变更文件列表（按 staged / unstaged / untracked flags 分组）。非仓库会抛 ErrNotARepo。
  async statusFiles(path: string): Promise<GitFileStatus[]> {
    const rows = await Binding.StatusFiles(path);
    return (rows ?? []).map(toFileStatus);
  },

  // 单文件 diff（staged=true 暂存区相对 HEAD / false 工作树相对暂存区）；复用 DiffView 渲染。
  async diffFile(path: string, file: string, staged: boolean): Promise<DiffResult> {
    return toDiff(await Binding.DiffFile(path, file, staged));
  },

  // 按文件绝对路径取工作树相对 HEAD 的 diff（后端自解析仓库根与相对路径）；供文件页就地查看改动。
  // 非仓库时后端透传 ErrNotARepo（调用方捕获后提示）。
  async diffFileAbs(absPath: string): Promise<DiffResult> {
    return toDiff(await Binding.DiffFileAbs(absPath));
  },

  // 暂存指定文件（git add）。
  async stage(path: string, files: string[]): Promise<void> {
    await Binding.Stage(path, files);
  },

  // 撤销暂存（git restore --staged）。
  async unstage(path: string, files: string[]): Promise<void> {
    await Binding.Unstage(path, files);
  },

  // 丢弃改动（已跟踪 restore / 未跟踪删文件）；破坏性，调用方须先 NativeDialogs.confirm。
  async discard(path: string, files: string[]): Promise<void> {
    await Binding.Discard(path, files);
  },

  // 提交暂存区内容（git commit -m），返回 git 摘要文本；无暂存内容/空消息会失败。
  async commit(path: string, message: string): Promise<string> {
    return Binding.Commit(path, message);
  },
} as const;
