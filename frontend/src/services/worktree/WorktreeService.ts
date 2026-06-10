// WorktreeService：自动 worktree（paseo 式会话隔离）的前端封装，包 worktreesvc bindings。
//
// 后端 CreateForProject 完成三步：.gitignore 守门（未忽略 .claude/.codex/.xkpanes 则补写并
// 单独提交一次）→ 集中根建树（分支 = 随机 slug）→ 登记进主仓所属 workspace。
// 仅桌面可用（worktree 创建是本机 git 操作；viewer 无此入口）。
import { Service as Binding } from '@bindings/xkpane/internal/services/worktreesvc';

// 一次自动创建的结果（对齐后端 worktreesvc.CreateResult）。
export interface WorktreeCreateResult {
  worktreePath: string; // 新 worktree 绝对路径（= 新会话 projectPath/cwd）
  branch: string; // 创建并检出的新分支名（= slug）
  repoRoot: string; // 主仓根（作 workspacePath 传 --add-dir）
  ignoreCommitted: boolean; // 本次是否补写了 .gitignore 并提交
  workspaceId: string; // 登记进的工作空间 id；'' = 未登记
}

export const WorktreeService = {
  // 为 projectPath 所在仓库自动创建隔离 worktree。非 git 仓库 / git 失败时 reject。
  async createForProject(projectPath: string): Promise<WorktreeCreateResult> {
    const res = await Binding.CreateForProject(projectPath);
    return {
      worktreePath: res.worktreePath ?? '',
      branch: res.branch ?? '',
      repoRoot: res.repoRoot ?? '',
      ignoreCommitted: res.ignoreCommitted ?? false,
      workspaceId: res.workspaceId ?? '',
    };
  },
} as const;
