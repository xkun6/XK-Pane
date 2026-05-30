// SessionScanService：按需扫盘列举本机可恢复的 CLI 会话（claude + codex）。
//
// 后端 internal/services/sessionscan 纯文件系统扫描（~/.claude/projects + ~/.codex/sessions），
// 解析 session id / cwd / 首条用户 prompt（截断）。供 WorkspaceTree 项目节点的会话子列表消费。
import { Service as Binding } from '@bindings/xkpane/internal/services/sessionscan';

// 对齐后端 sessionscan.ResumeSession。
export interface ResumeSession {
  id: string;
  source: 'claude' | 'codex' | (string & {});
  projectPath: string;
  modifiedAt: number; // unix 秒
  filePath: string;
  description: string;
}

export const SessionScanService = {
  // 列举指定项目路径下 claude + codex 的历史会话，按 mtime 倒序。limit<=0 不限。
  async listProjectSessions(projectPath: string, limit = 0): Promise<ResumeSession[]> {
    const rows = await Binding.ListProjectSessions(projectPath, limit);
    return (rows ?? []).map<ResumeSession>((r) => ({
      id: r.id ?? '',
      source: (r.source ?? '') as ResumeSession['source'],
      projectPath: r.projectPath ?? '',
      modifiedAt: Number(r.modifiedAt ?? 0),
      filePath: r.filePath ?? '',
      description: r.description ?? '',
    }));
  },
} as const;
