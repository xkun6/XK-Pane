// SessionsPage ViewModel —— 移植 cc-pane SessionsView 的数据装配（活跃会话 + 最近启动）。
// 复用既有数据源：useTerminal（面板树 + 启动 + 聚焦）/ useTerminalStatuses（活跃状态）/
// LaunchHistoryService（最近启动）/ useRouter（点会话跳终端页）。
// 不返回 JSX、不持翻译串（分组的「未分组」标签 + 确认文案由 View 传入，遵 MVVM 铁律）。
import { useCallback, useEffect, useState } from 'react';
import { LaunchHistoryService, NativeDialogs, type LaunchRecord, type TerminalStatusKind } from '@/services';
import { useRouter } from '@/router';
import { useTerminal, useTerminalStatuses } from '@/terminal';
import type { PaneNode, Panel } from '@/terminal';

const HISTORY_LIMIT = 30;

/** 一条活跃会话（从面板树 + 状态 store 派生）。 */
export interface ActiveSession {
  tabId: string;
  paneId: string;
  title: string;
  sessionId: string;
  status: TerminalStatusKind;
}

export interface ClearStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

export interface SessionsPageModel {
  activeSessions: ActiveSession[];
  launchHistory: LaunchRecord[];
  focusSession: (paneId: string, tabId: string) => void;
  resume: (record: LaunchRecord) => void;
  deleteRecord: (id: number) => Promise<void>;
  clearHistory: (s: ClearStrings) => Promise<void>;
}

/** 递归收集所有 Panel 节点（移植 cc-pane SessionsView.getAllPanels）。 */
function getAllPanels(pane: PaneNode): Panel[] {
  if (pane.type === 'panel') return [pane];
  return pane.children.flatMap(getAllPanels);
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export function useSessionsPage(): SessionsPageModel {
  const { rootPane, setActiveTab, setActivePanel, launchProject } = useTerminal();
  const { statuses } = useTerminalStatuses();
  const { navigate } = useRouter();
  const [launchHistory, setLaunchHistory] = useState<LaunchRecord[]>([]);

  const fetchHistory = useCallback(async (): Promise<void> => {
    try {
      setLaunchHistory(await LaunchHistoryService.list(HISTORY_LIMIT));
    } catch {
      // 拉取失败静默：空列表即反馈。
    }
  }, []);

  useEffect(() => {
    void fetchHistory();
  }, [fetchHistory]);

  // 活跃会话：树中所有「有 sessionId 的终端 Tab」+ 状态 store 派生状态。
  const activeSessions: ActiveSession[] = getAllPanels(rootPane).flatMap((panel) =>
    panel.tabs
      .filter((tab) => tab.sessionId && tab.contentType === 'terminal')
      .map((tab) => ({
        tabId: tab.id,
        paneId: panel.id,
        title: tab.title,
        sessionId: tab.sessionId as string,
        status: statuses[tab.sessionId as string] ?? 'idle',
      })),
  );

  const focusSession = useCallback(
    (paneId: string, tabId: string): void => {
      setActivePanel(paneId);
      setActiveTab(paneId, tabId);
      navigate('terminal');
    },
    [setActivePanel, setActiveTab, navigate],
  );

  const resume = useCallback(
    (record: LaunchRecord): void => {
      if (!record.resumeSessionId) return;
      launchProject({
        projectId: record.projectId,
        projectName: record.projectName,
        projectPath: record.projectPath,
        cliTool: record.cliTool,
        resumeId: record.resumeSessionId,
      });
      navigate('terminal');
    },
    [launchProject, navigate],
  );

  const deleteRecord = useCallback(
    async (id: number): Promise<void> => {
      try {
        await LaunchHistoryService.delete(id);
        await fetchHistory();
      } catch {
        // 静默：列表刷新即反馈。
      }
    },
    [fetchHistory],
  );

  const clearHistory = useCallback(
    async (s: ClearStrings): Promise<void> => {
      const ok = await NativeDialogs.confirm({
        title: s.confirmTitle,
        message: s.confirmMessage,
        okLabel: s.okLabel,
        cancelLabel: s.cancelLabel,
      });
      if (!ok) return;
      try {
        await LaunchHistoryService.clear();
        setLaunchHistory([]);
      } catch (e) {
        await NativeDialogs.error(s.errorTitle, messageOf(e));
      }
    },
    [],
  );

  return { activeSessions, launchHistory, focusSession, resume, deleteRecord, clearHistory };
}
