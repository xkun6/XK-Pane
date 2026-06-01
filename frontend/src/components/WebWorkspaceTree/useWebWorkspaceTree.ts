// WebWorkspaceTree 的 ViewModel —— 仅 Web（viewer）用。把桌面 WorkspaceTree 的「按工作区
// 浏览 + 恢复历史会话 + 新建会话」子集搬到浏览器/手机：
//   - 工作区树走 WorkspaceService.list()（viewer → GET /api/workspaces）
//   - 项目下历史会话走 SessionScanService.listProjectSessions()（viewer → GET /api/sessions/scan）
//   - 恢复 / 新建都走 OrchestratorService.launch()（POST /api/launch）→ 后端广播 orchestrator:launch
//     → 桌面与本端同步开窗附着同 sid（只读镜像，输入权属桌面 owner）。
//
// MVVM：不返回 JSX、不持翻译串（错误文案里的后端原文属「数据」，可持有）。经 services 层通信，
// 不直接 import bindings / httpClient。移动端无右键 → 行尾 ⋮ 锚定菜单（菜单状态在此，渲染在 View）。
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  OrchestratorService,
  SessionScanService,
  TerminalService,
  WorkspaceService,
  type ResumeSession,
  type SessionInfo,
  type Workspace,
  type WorkspaceProject,
} from '@/services';
import { copyText } from '@/shared/clipboard';
import { useTerminal } from '@/terminal';

// basename：取路径最后一段作显示名（兼容 / 与 \）。
function basename(path: string): string {
  return path.split(/[/\\]/).filter(Boolean).pop() || path;
}

// Web 启动可选 CLI（桌面 CliToolService 走 bindings，浏览器不可用 → 最小静态集，对齐 WebLauncher）。
const CLI_TOOLS = ['claude', 'codex', 'reclaude'] as const;

// 某项目历史会话的加载态（按项目节点 id 缓存，展开时按需拉取）。
export type SessionState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; sessions: ResumeSession[] };

// 锚定式菜单状态：project（新建会话 / 复制路径）或 session（恢复 / 以 reclaude 恢复 / 复制 id）。
export type TreeMenu =
  | { kind: 'project'; anchorEl: HTMLElement; project: WorkspaceProject; label: string }
  | {
      kind: 'session';
      anchorEl: HTMLElement;
      session: ResumeSession;
      projectPath: string;
    };

export interface NewSessionDialogState {
  open: boolean;
  projectPath: string;
  projectLabel: string;
  prompt: string;
  cliTool: string;
  launching: boolean;
  error: string | null;
}

export interface UseWebWorkspaceTree {
  // 运行中会话（含桌面手点开的）：消费 GET /api/sessions，点击 attach 只读镜像（P7 M5）。
  activeSessions: SessionInfo[];
  activeLoading: boolean;
  activeFailed: boolean;
  reloadActive: () => void;
  attach: (session: SessionInfo) => void;

  workspaces: Workspace[];
  loading: boolean;
  loadFailed: boolean;
  reload: () => void;

  expandedWorkspaceId: string | null;
  toggleWorkspace: (id: string) => void;
  expandedProjectIds: ReadonlySet<string>;
  toggleProject: (project: WorkspaceProject) => void;
  sessionStateFor: (projectId: string) => SessionState | undefined;

  menu: TreeMenu | null;
  openMenu: (menu: TreeMenu) => void;
  closeMenu: () => void;

  resume: (session: ResumeSession, projectPath: string, cliToolOverride?: string) => void;
  copyId: (session: ResumeSession) => void;
  copyPath: (project: WorkspaceProject) => void;

  dialog: NewSessionDialogState;
  cliToolOptions: readonly string[];
  openNewSession: (projectPath: string, projectLabel: string) => void;
  closeDialog: () => void;
  setPrompt: (s: string) => void;
  setCliTool: (s: string) => void;
  canLaunch: boolean;
  launch: () => void;

  // 动作反馈（恢复失败 / 复制结果）—— 非安全上下文复制可能失败，必须有反馈。文案 key 在 View。
  notice: NoticeKey | null;
  clearNotice: () => void;
}

// notice 用 key 而非翻译串（MVVM：VM 不持翻译串），View 据此调 t()。
export type NoticeKey = 'copied' | 'copyFailed' | 'resumeFailed';

// resume id 合法性：非空且不含 ':'（裸 id 才能透传 CLI；对照桌面 useSessionSubList）。
function isValidResumeId(id: string): boolean {
  return id !== '' && !id.includes(':');
}

export function useWebWorkspaceTree(opts?: { onSelect?: () => void }): UseWebWorkspaceTree {
  const { newTab, activePaneId } = useTerminal();
  // onSelect 用 ref 持有：在 attach/resume/launch 里调用而不进 useCallback 依赖（避免陈旧闭包）。
  const onSelectRef = useRef(opts?.onSelect);
  onSelectRef.current = opts?.onSelect;
  const [activeSessions, setActiveSessions] = useState<SessionInfo[]>([]);
  const [activeLoading, setActiveLoading] = useState(true);
  const [activeFailed, setActiveFailed] = useState(false);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [expandedWorkspaceId, setExpandedWorkspaceId] = useState<string | null>(null);
  const [expandedProjectIds, setExpandedProjectIds] = useState<Set<string>>(new Set());
  const [sessionStates, setSessionStates] = useState<Record<string, SessionState>>({});
  const [menu, setMenu] = useState<TreeMenu | null>(null);
  const [notice, setNotice] = useState<NoticeKey | null>(null);
  const [dialog, setDialog] = useState<NewSessionDialogState>({
    open: false,
    projectPath: '',
    projectLabel: '',
    prompt: '',
    cliTool: CLI_TOOLS[0],
    launching: false,
    error: null,
  });

  const reload = useCallback(() => {
    setLoading(true);
    setLoadFailed(false);
    WorkspaceService.list()
      .then((rows) => setWorkspaces(rows))
      .catch(() => setLoadFailed(true))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  // 运行中会话列表（含桌面手点开的会话；只有这条路径 web 才能看到非 Web/MCP 发起的会话）。
  const reloadActive = useCallback(() => {
    setActiveLoading(true);
    setActiveFailed(false);
    TerminalService.list()
      .then((rows) => setActiveSessions(rows))
      .catch(() => setActiveFailed(true))
      .finally(() => setActiveLoading(false));
  }, []);

  useEffect(() => {
    reloadActive();
  }, [reloadActive]);

  // attach：在活动面板开一个绑定该已存在 sid 的只读面板（reconcile 在 viewer 不跑，不会重复建会话）。
  // TerminalView 经 attachSessionOutput 先重放历史画面再切 live（P7 M4）。
  const attach = useCallback(
    (session: SessionInfo) => {
      const name = basename(session.cwd);
      const title =
        session.cliTool && session.cliTool !== 'none' ? `${name} (${session.cliTool})` : name;
      newTab(activePaneId, {
        projectId: '',
        projectPath: session.cwd,
        cliTool: session.cliTool || undefined,
        title,
        sessionId: session.id,
      });
      onSelectRef.current?.();
    },
    [newTab, activePaneId],
  );

  const toggleWorkspace = useCallback((id: string) => {
    setExpandedWorkspaceId((cur) => (cur === id ? null : id));
  }, []);

  const loadSessions = useCallback((projectId: string, projectPath: string) => {
    setSessionStates((s) => ({ ...s, [projectId]: { status: 'loading' } }));
    SessionScanService.listProjectSessions(projectPath)
      .then((list) =>
        setSessionStates((s) => ({ ...s, [projectId]: { status: 'ready', sessions: list } })),
      )
      .catch(() => setSessionStates((s) => ({ ...s, [projectId]: { status: 'error' } })));
  }, []);

  const toggleProject = useCallback(
    (project: WorkspaceProject) => {
      const willExpand = !expandedProjectIds.has(project.id);
      setExpandedProjectIds((cur) => {
        const next = new Set(cur);
        if (next.has(project.id)) next.delete(project.id);
        else next.add(project.id);
        return next;
      });
      if (willExpand && !sessionStates[project.id]) {
        loadSessions(project.id, project.path);
      }
    },
    [expandedProjectIds, sessionStates, loadSessions],
  );

  const sessionStateFor = useCallback(
    (projectId: string): SessionState | undefined => sessionStates[projectId],
    [sessionStates],
  );

  const openMenu = useCallback((m: TreeMenu) => setMenu(m), []);
  const closeMenu = useCallback(() => setMenu(null), []);
  const clearNotice = useCallback(() => setNotice(null), []);

  const resume = useCallback(
    (session: ResumeSession, projectPath: string, cliToolOverride?: string) => {
      setMenu(null);
      if (!isValidResumeId(session.id)) return;
      // 恢复 = launch 带 resumeId（与 prompt 互斥）。窗口由 orchestrator:launch 广播在终端页开出。
      OrchestratorService.launch({
        projectPath,
        resumeId: session.id,
        cliTool: cliToolOverride ?? session.source,
      }).catch(() => setNotice('resumeFailed'));
      onSelectRef.current?.();
    },
    [],
  );

  const copyId = useCallback((session: ResumeSession) => {
    setMenu(null);
    void copyText(session.id).then((ok) => setNotice(ok ? 'copied' : 'copyFailed'));
  }, []);

  const copyPath = useCallback((project: WorkspaceProject) => {
    setMenu(null);
    void copyText(project.path).then((ok) => setNotice(ok ? 'copied' : 'copyFailed'));
  }, []);

  const openNewSession = useCallback((projectPath: string, projectLabel: string) => {
    setMenu(null);
    setDialog({
      open: true,
      projectPath,
      projectLabel,
      prompt: '',
      cliTool: CLI_TOOLS[0],
      launching: false,
      error: null,
    });
  }, []);

  const closeDialog = useCallback(() => setDialog((d) => ({ ...d, open: false })), []);
  const setPrompt = useCallback((s: string) => setDialog((d) => ({ ...d, prompt: s })), []);
  const setCliTool = useCallback((s: string) => setDialog((d) => ({ ...d, cliTool: s })), []);

  const canLaunch =
    dialog.projectPath.trim() !== '' && dialog.prompt.trim() !== '' && !dialog.launching;

  const launch = useCallback(() => {
    if (dialog.projectPath.trim() === '' || dialog.prompt.trim() === '') return;
    setDialog((d) => ({ ...d, launching: true, error: null }));
    OrchestratorService.launch({
      projectPath: dialog.projectPath,
      prompt: dialog.prompt,
      cliTool: dialog.cliTool,
    })
      .then(() => {
        setDialog((d) => ({ ...d, open: false, prompt: '', launching: false }));
        onSelectRef.current?.();
      })
      .catch((e: unknown) =>
        setDialog((d) => ({
          ...d,
          launching: false,
          error: e instanceof Error ? e.message : String(e),
        })),
      );
  }, [dialog.projectPath, dialog.prompt, dialog.cliTool]);

  return {
    activeSessions,
    activeLoading,
    activeFailed,
    reloadActive,
    attach,
    workspaces,
    loading,
    loadFailed,
    reload,
    expandedWorkspaceId,
    toggleWorkspace,
    expandedProjectIds,
    toggleProject,
    sessionStateFor,
    menu,
    openMenu,
    closeMenu,
    resume,
    copyId,
    copyPath,
    dialog,
    cliToolOptions: CLI_TOOLS,
    openNewSession,
    closeDialog,
    setPrompt,
    setCliTool,
    canLaunch,
    launch,
    notice,
    clearNotice,
  };
}
