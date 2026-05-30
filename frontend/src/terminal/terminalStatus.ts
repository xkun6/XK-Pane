// 终端会话状态映射的纯函数 —— 供 TerminalStatusContext 维护 sessionId→status。
// 无副作用，便于 Vitest 覆盖（事件订阅 / setState 在 Context 里）。
//
// 后端目前只发两种状态（terminal.go）：首次输出 → 'active'；退出 → 'exited'。
// 'idle' / 'waiting_input' 是预留枚举值（后端暂不发）。活跃指示只在 'active' 点亮。
import type { TerminalStatusKind } from '@/services';

/** sessionId → 最新状态。'exited' 的会话从 map 删除（= 不再活跃），故 map 仅含在跑会话。 */
export type StatusMap = Readonly<Record<string, TerminalStatusKind>>;

export const emptyStatusMap: StatusMap = {};

/**
 * 应用一条状态变更，返回新 map（不可变）。
 * - 'exited'：删除该 session 条目（退出即不活跃，避免 map 无界增长）。
 * - 其他：写入最新状态。
 * 状态相同则返回原 map（避免无谓 re-render）。
 */
export function applyStatus(
  map: StatusMap,
  update: { sessionId: string; status: TerminalStatusKind },
): StatusMap {
  const { sessionId, status } = update;
  if (status === 'exited') {
    if (!(sessionId in map)) return map;
    const next = { ...map };
    delete next[sessionId];
    return next;
  }
  if (map[sessionId] === status) return map;
  return { ...map, [sessionId]: status };
}

/** 该会话是否处于活跃态（有 PTY 输出且未退出）。 */
export function isSessionActive(map: StatusMap, sessionId: string | null | undefined): boolean {
  return !!sessionId && map[sessionId] === 'active';
}
