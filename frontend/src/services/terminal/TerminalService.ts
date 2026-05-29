// TerminalService：内置终端会话的前端入口。
//
// 后端在 PTY 里启动选定 CLI（claude/codex/...）或裸 shell，PTY 原始字节流
// （含 ANSI / 光标控制序列）经 Wails Events 批量推送：
//   - terminal:output  {sessionId, data}                  → 直接喂 xterm.write(data)
//   - terminal:status  {sessionId, status, lastOutputAt, pid}
//   - terminal:exit    {sessionId, exitCode}
//
// 与 subprocess 不同：terminal 事件名是**固定单名**，sessionId 在 payload 里
// （events.go 用 RegisterEvent[TerminalOutputPayload]("terminal:output")），
// 因此订阅按 payload.sessionId 过滤。业务方用 onOutput / onStatus / onExit 订阅，
// 不直接接 Events；每个订阅返回取消函数，务必在 useEffect 里 return 它。
import { Events } from '@wailsio/runtime';
import { Service as Binding } from '@bindings/xkpane/internal/services/terminal';

// CreateSpec 对齐后端 terminal.CreateSpec（路径字段铁律 D7：projectPath = PTY cwd）。
export interface CreateSpec {
  projectPath: string;
  workspacePath?: string; // 仅多目录 CLI 且 ≠projectPath 时拼 --add-dir；缺省 ''
  cliTool?: string;       // 适配器 id；'' | 'none' = 裸 shell
  cols?: number;          // 0 取 ptyx 默认；拿到真实尺寸后应立即 resize
  rows?: number;
  initialPrompt?: string; // 初始 prompt（CLI 位置参数）；缺省 ''
}

export interface SessionInfo {
  id: string;
  pid: number;
  cwd: string;
  cliTool: string;
}

export interface TerminalOutput {
  sessionId: string;
  data: string;
}

export type TerminalStatusKind = 'active' | 'idle' | 'waiting_input' | 'exited';

export interface TerminalStatus {
  sessionId: string;
  status: TerminalStatusKind;
  lastOutputAt: number; // unix milli
  pid: number;
}

export interface TerminalExit {
  sessionId: string;
  exitCode: number;
}

// 与 internal/events/events.go 的事件名常量保持同步。
const EVENT = {
  output: 'terminal:output',
  status: 'terminal:status',
  exit: 'terminal:exit',
} as const;

// onSession 订阅某事件，按 payload.sessionId 过滤后回调；返回取消函数。
function onSession<T extends { sessionId: string }>(
  event: string,
  sessionId: string,
  handler: (data: T) => void,
): () => void {
  const off = Events.On(event, (e) => {
    const data = e?.data as T | undefined;
    if (data && data.sessionId === sessionId) handler(data);
  });
  return () => {
    if (typeof off === 'function') off();
  };
}

export const TerminalService = {
  // 启动一个 PTY 会话，返回 sessionId。后续输出经 onOutput 流式到达。
  async create(spec: CreateSpec): Promise<string> {
    return Binding.CreateSession({
      projectPath: spec.projectPath,
      workspacePath: spec.workspacePath ?? '',
      cliTool: spec.cliTool ?? '',
      cols: spec.cols ?? 0,
      rows: spec.rows ?? 0,
      initialPrompt: spec.initialPrompt ?? '',
    });
  },

  // 写用户输入（键入字节，含回车 '\r' / Ctrl 序列）到 PTY。
  async write(sessionId: string, data: string): Promise<void> {
    await Binding.WriteInput(sessionId, data);
  },

  // 调整 PTY 尺寸（cols=宽, rows=高）。xterm fit 后应调用。
  async resize(sessionId: string, cols: number, rows: number): Promise<void> {
    await Binding.Resize(sessionId, cols, rows);
  },

  // 硬终止会话（幂等：未知 / 已退出返回成功）。
  async kill(sessionId: string): Promise<void> {
    await Binding.Kill(sessionId);
  },

  // 当前活跃会话快照。
  async list(): Promise<SessionInfo[]> {
    const list = await Binding.List();
    return (list ?? []).map<SessionInfo>((it) => ({
      id: it.id ?? '',
      pid: Number(it.pid ?? 0),
      cwd: it.cwd ?? '',
      cliTool: it.cliTool ?? '',
    }));
  },

  // 订阅指定会话的 PTY 输出（原始字节字符串，直接 xterm.write）。
  onOutput(sessionId: string, handler: (data: string) => void): () => void {
    return onSession<TerminalOutput>(EVENT.output, sessionId, (d) => {
      if (typeof d.data === 'string') handler(d.data);
    });
  },

  // 订阅指定会话的状态变更（active / idle / waiting_input / exited）。
  onStatus(sessionId: string, handler: (status: TerminalStatus) => void): () => void {
    return onSession<TerminalStatus>(EVENT.status, sessionId, (d) => {
      handler({
        sessionId: d.sessionId,
        status: (d.status ?? 'active') as TerminalStatusKind,
        lastOutputAt: Number(d.lastOutputAt ?? 0),
        pid: Number(d.pid ?? 0),
      });
    });
  },

  // 订阅指定会话的退出事件。
  onExit(sessionId: string, handler: (info: TerminalExit) => void): () => void {
    return onSession<TerminalExit>(EVENT.exit, sessionId, (d) => {
      handler({
        sessionId: d.sessionId,
        exitCode: Number(d.exitCode ?? 0),
      });
    });
  },
} as const;
