// SessionScanService：按需扫盘列举本机可恢复的 CLI 会话（claude + codex + grok）。
//
// 后端 internal/services/sessionscan 纯文件系统扫描
// （~/.claude/projects + ~/.codex/sessions + ~/.grok/sessions），
// 解析 session id / cwd / 首条用户 prompt 或标题（截断）。供 WorkspaceTree 项目节点的会话子列表消费。
import { Service as Binding } from '@bindings/xkpane/internal/services/sessionscan';
import { httpClient, isDesktop } from '@/services/transport';

// 对齐后端 sessionscan.ResumeSession。
export interface ResumeSession {
  id: string;
  source: 'claude' | 'codex' | 'grok' | (string & {});
  projectPath: string;
  modifiedAt: number; // unix 秒
  filePath: string;
  description: string;
}

// 原始扫盘行：桌面 bindings 与 Web GET /api/sessions/scan 同构（同一 Go 结构的 camelCase）。
interface RawResumeSession {
  id?: string;
  source?: string;
  projectPath?: string;
  modifiedAt?: number;
  filePath?: string;
  description?: string;
}

export const SessionScanService = {
  // 列举指定项目路径下 claude + codex + grok 的历史会话，按 mtime 倒序。limit<=0 不限。
  // 桌面走 bindings 直接扫盘；Web（viewer）走网关 GET /api/sessions/scan（网关在 PC 上扫盘后回传）。
  async listProjectSessions(projectPath: string, limit = 0): Promise<ResumeSession[]> {
    const rows = ((isDesktop
      ? await Binding.ListProjectSessions(projectPath, limit)
      : await scanViaHttp(projectPath, limit)) ?? []) as RawResumeSession[];
    return rows.map<ResumeSession>((r) => ({
      id: r.id ?? '',
      source: (r.source ?? '') as ResumeSession['source'],
      projectPath: r.projectPath ?? '',
      modifiedAt: Number(r.modifiedAt ?? 0),
      filePath: r.filePath ?? '',
      description: r.description ?? '',
    }));
  },
} as const;

// viewer 扫盘：GET /api/sessions/scan?projectPath=&limit=（桌面不会走到此分支）。
async function scanViaHttp(projectPath: string, limit: number): Promise<RawResumeSession[]> {
  const qs = new URLSearchParams({ projectPath });
  if (limit > 0) qs.set('limit', String(limit));
  const res = await httpClient.get<{ sessions?: RawResumeSession[] }>(
    `/api/sessions/scan?${qs.toString()}`,
  );
  return res.sessions ?? [];
}
