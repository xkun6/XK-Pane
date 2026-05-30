// 终端会话状态 store —— 单点订阅 terminal:status 事件，维护 sessionId→status 映射。
//
// 为什么独立于 TerminalContext：状态变更频繁（每个会话首次输出 / 退出都发），若塞进
// TerminalContext 会让整棵面板树跟着 re-render；独立轻量 Context 只让真正读状态的组件
// （WorkspaceTree 活跃指示 / SessionsView 活跃会话）订阅。
//
// 数据源：TerminalService.onAnyStatus（全量，不按 sessionId 过滤）。后端发 'active'（首次
// 输出）/ 'exited'（退出），见 terminal.go。
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { TerminalService } from '@/services';
import { applyStatus, emptyStatusMap, isSessionActive, type StatusMap } from './terminalStatus';

export interface TerminalStatusContextValue {
  /** sessionId → 最新状态（仅含在跑会话；退出即移除）。 */
  statuses: StatusMap;
  /** 该会话是否活跃（'active'）。 */
  isActive: (sessionId: string | null | undefined) => boolean;
}

const TerminalStatusContext = createContext<TerminalStatusContextValue | null>(null);

export const TerminalStatusProvider = ({ children }: { children: ReactNode }) => {
  const [statuses, setStatuses] = useState<StatusMap>(emptyStatusMap);

  useEffect(() => {
    const off = TerminalService.onAnyStatus((s) => {
      setStatuses((prev) => applyStatus(prev, { sessionId: s.sessionId, status: s.status }));
    });
    return off;
  }, []);

  const value = useMemo<TerminalStatusContextValue>(
    () => ({
      statuses,
      isActive: (sessionId) => isSessionActive(statuses, sessionId),
    }),
    [statuses],
  );

  return (
    <TerminalStatusContext.Provider value={value}>{children}</TerminalStatusContext.Provider>
  );
};

export const useTerminalStatuses = (): TerminalStatusContextValue => {
  const ctx = useContext(TerminalStatusContext);
  if (!ctx) {
    throw new Error('useTerminalStatuses must be used inside <TerminalStatusProvider>');
  }
  return ctx;
};
