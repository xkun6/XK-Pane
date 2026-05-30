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
} from '@bindings/xkpane/internal/utils/gitx/models';

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
} as const;
