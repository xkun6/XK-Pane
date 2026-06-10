// TerminalService：内置终端会话的前端入口。
//
// 后端在 PTY 里启动选定 CLI（claude/codex/...）或裸 shell，PTY 原始字节流（含 ANSI / 光标
// 控制序列）经事件批量推送：terminal:output / status / exit（payload 带 sessionId）。
//
// transport 抽象（P7 M2）：
//   - 事件订阅经 eventTransport（桌面 Wails 总线 / 浏览器 /ws 镜像），按 payload.sessionId 过滤。
//   - RPC 按 isDesktop 分流：桌面走类型安全 bindings；浏览器走同源 HTTP（fail-closed 白名单）。
//     M3 为只读镜像 —— 浏览器的 write/resize/kill 暂为 no-op（输入/尺寸权属/终止 = M5/M6），
//     仅 list 走 GET /api/sessions、创建会话改由「启动」（OrchestratorService.launch）。
import { Service as Binding } from '@bindings/xkpane/internal/services/terminal';
import { AppEvents } from '@/shared/events';
import { eventTransport, httpClient, isDesktop } from '@/services/transport';

// SSHSpec 对齐后端 terminal.SSHSpec（RuntimeKind=='ssh' 时的连接信息）。
// 去规范化字段通常由前端从所选机器 DTO 拷入；machineId 仅供后端密码自动应答反查。
export interface SSHSpec {
  machineId?: string;
  host: string;
  port?: number;       // 缺省 22
  user?: string;
  remotePath?: string; // 远端默认 cd 目录；'~' / '' 跳过 cd
  identityFile?: string;
  authMethod?: string; // password | key | agent
}

// CreateSpec 对齐后端 terminal.CreateSpec（路径字段铁律 D7：projectPath = PTY cwd）。
export interface CreateSpec {
  projectPath: string;
  workspacePath?: string; // 仅多目录 CLI 且 ≠projectPath 时拼 --add-dir；缺省 ''
  cliTool?: string;       // 适配器 id；'' | 'none' = 裸 shell
  providerId?: string;    // 选定 Provider id；'' = 不注入 provider env
  cols?: number;          // 0 取 ptyx 默认；拿到真实尺寸后应立即 resize
  rows?: number;
  initialPrompt?: string; // 初始 prompt（CLI 位置参数）；缺省 ''
  resumeId?: string;      // 恢复会话 id（明文 → CLI resume）；缺省 '' = 新会话
  runtimeKind?: string;   // ''/'local'（默认）| 'ssh'（本地 ssh 客户端拉起远程会话）
  ssh?: SSHSpec;          // runtimeKind=='ssh' 时必填
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
  seq?: number; // 会话内单调 chunk 序号（P7 M4）：attach 重放据此对 live 去重，不漏不重
}

// RawReplay 是 attach 重放快照（P7 M4，对齐后端 terminal.RawReplay）。
export interface RawReplay {
  data: string; // 原始 ANSI，直接 xterm.write 重建画面
  lastSeq: number; // 快照覆盖到的最大 seq；live 中 seq ≤ lastSeq 的 chunk 应丢弃
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

// onSession 订阅某事件，按 payload.sessionId 过滤后回调；返回取消函数。
// 经 eventTransport（已解包 payload），桌面 / 浏览器同一套代码。
function onSession<T extends { sessionId: string }>(
  event: string,
  sessionId: string,
  handler: (data: T) => void,
): () => void {
  return eventTransport.on<T>(event, (data) => {
    if (data && data.sessionId === sessionId) handler(data);
  });
}

// 浏览器 /api/sessions 返回的会话快照（对齐后端 terminal.SessionSnapshot）。
interface SessionSnapshotDTO {
  id?: string;
  pid?: number;
  cwd?: string;
  cliTool?: string;
}

export const TerminalService = {
  // 启动一个 PTY 会话，返回 sessionId。后续输出经 onOutput 流式到达。
  // 浏览器（viewer）不直接建会话 —— 用「启动」（OrchestratorService.launch → POST /api/launch）
  // 经 orchestrator:launch 广播在两端开窗附着（reconcile 在 viewer 不跑，见 TerminalContext）。
  async create(spec: CreateSpec): Promise<string> {
    if (!isDesktop) {
      return Promise.reject(new Error('web 端不直接创建会话，请用启动（launch）'));
    }
    return Binding.CreateSession({
      projectPath: spec.projectPath,
      workspacePath: spec.workspacePath ?? '',
      cliTool: spec.cliTool ?? '',
      providerId: spec.providerId ?? '',
      cols: spec.cols ?? 0,
      rows: spec.rows ?? 0,
      initialPrompt: spec.initialPrompt ?? '',
      resumeId: spec.resumeId ?? '',
      runtimeKind: spec.runtimeKind ?? 'local',
      ssh: spec.ssh
        ? {
            machineId: spec.ssh.machineId ?? '',
            host: spec.ssh.host,
            port: spec.ssh.port ?? 22,
            user: spec.ssh.user ?? '',
            remotePath: spec.ssh.remotePath ?? '',
            identityFile: spec.ssh.identityFile ?? '',
            authMethod: spec.ssh.authMethod ?? '',
          }
        : null,
    });
  },

  // 写用户输入（键入字节，含回车 '\r' / Ctrl 序列）到 PTY。
  // 桌面走 binding；浏览器（viewer）走 POST /api/sessions/input（P7 M6 双向输入）。
  // TUI 自动应答（CPR/DA/OSC）的去重在 useTerminalView：viewer 不发应答、由 owner 桌面应答。
  async write(sessionId: string, data: string): Promise<void> {
    if (!isDesktop) {
      await httpClient.post('/api/sessions/input', { sessionId, data });
      return;
    }
    await Binding.WriteInput(sessionId, data);
  },

  // 提交一句输入给会话内 CLI：两段式写入（文本 → 按长度延时 → 单独 '\r'）。
  // 绝不能把 text+'\r' 当一整块写：codex 的 paste-burst 判定（字符间隔 <8ms 视为粘贴）
  // 会把尾部 '\r' 当换行而非提交；Ink/TUI 也需要时间消化长文本（同后端 orchestrator
  // submit_to_session 的 computeEnterDelay 范式）。多行文本包 bracketed-paste 序列
  // （与终端粘贴同通道，claude/codex 都把其中换行当正文）。busy 时直发也安全：
  // claude/codex TUI 原生支持运行中提交入队（steer/queue），无需前端排队。
  async submit(sessionId: string, text: string): Promise<void> {
    const clean = text.replace(/\r\n?/g, '\n'); // 正文里的 \r 统一成 \n（防被当作提交键）
    const payload = clean.includes('\n') ? `\x1b[200~${clean}\x1b[201~` : clean;
    await TerminalService.write(sessionId, payload);
    const delayMs = Math.min(200 + Math.floor(payload.length / 512) * 30, 5000);
    await new Promise((resolve) => setTimeout(resolve, delayMs));
    await TerminalService.write(sessionId, '\r');
  },

  // 调整 PTY 尺寸（cols=宽, rows=高）。浏览器：no-op（resize 权属 = owner/桌面，M6）。
  async resize(sessionId: string, cols: number, rows: number): Promise<void> {
    if (!isDesktop) return;
    await Binding.Resize(sessionId, cols, rows);
  },

  // 硬终止会话（幂等）。浏览器：no-op（viewer 不经 reconcile kill；显式「终止会话」= M5）。
  async kill(sessionId: string): Promise<void> {
    if (!isDesktop) return;
    await Binding.Kill(sessionId);
  },

  // 当前活跃会话快照。桌面走 bindings；浏览器走 GET /api/sessions。
  async list(): Promise<SessionInfo[]> {
    if (!isDesktop) {
      const res = await httpClient.get<{ sessions?: SessionSnapshotDTO[] }>('/api/sessions');
      return (res.sessions ?? []).map<SessionInfo>((it) => ({
        id: it.id ?? '',
        pid: Number(it.pid ?? 0),
        cwd: it.cwd ?? '',
        cliTool: it.cliTool ?? '',
      }));
    }
    const list = await Binding.List();
    return (list ?? []).map<SessionInfo>((it) => ({
      id: it.id ?? '',
      pid: Number(it.pid ?? 0),
      cwd: it.cwd ?? '',
      cliTool: it.cliTool ?? '',
    }));
  },

  // 订阅指定会话的 PTY 输出（原始字节字符串，直接 xterm.write）。
  // handler 第二参 seq = 会话内单调 chunk 序号（attach 重放去重用）；只关心数据的调用方忽略即可。
  onOutput(sessionId: string, handler: (data: string, seq: number) => void): () => void {
    return onSession<TerminalOutput>(AppEvents.TerminalOutput, sessionId, (d) => {
      if (typeof d.data === 'string') handler(d.data, Number(d.seq ?? 0));
    });
  },

  // attach 重放快照（P7 M4）。浏览器走 GET /api/sessions/replay；桌面暂返回空
  // （桌面 attach 重放 = 后续；其 SessionReplay binding 待 regen，M4 无桌面消费方）。
  async replay(sessionId: string): Promise<RawReplay> {
    if (!isDesktop) {
      const res = await httpClient.get<{ data?: string; lastSeq?: number }>(
        `/api/sessions/replay?sessionId=${encodeURIComponent(sessionId)}`,
      );
      return { data: res.data ?? '', lastSeq: Number(res.lastSeq ?? 0) };
    }
    return { data: '', lastSeq: 0 };
  },

  // 订阅指定会话的状态变更（active / idle / waiting_input / exited）。
  onStatus(sessionId: string, handler: (status: TerminalStatus) => void): () => void {
    return onSession<TerminalStatus>(AppEvents.TerminalStatus, sessionId, (d) => {
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
    return onSession<TerminalExit>(AppEvents.TerminalExit, sessionId, (d) => {
      handler({
        sessionId: d.sessionId,
        exitCode: Number(d.exitCode ?? 0),
      });
    });
  },
} as const;

// attachSessionOutput：把一个**已存在**会话的输出接到 write 回调，attach 时重建历史画面再切
// live（P7 M4，供 M5 attach 面板的 xterm 消费）。顺序保证不漏不重：
//   ① 先订阅 live 并**缓冲**（不立即写）—— 保证拉 replay 期间到达的 chunk 不丢；
//   ② 拉 replay 快照 → xterm.write 重建画面 → 记 lastSeq；
//   ③ 回放缓冲中 seq > lastSeq 的 chunk（已被快照覆盖的 seq ≤ lastSeq 丢弃，防重复）；
//   ④ 切 live：后续 chunk 直接写（仍按 seq > lastSeq 守门）。
// replay 失败则直接切 live（无历史画面但不漏后续）。返回取消订阅函数。
export function attachSessionOutput(
  sessionId: string,
  write: (data: string) => void,
): () => void {
  let lastSeq = -1;
  let phase: 'replaying' | 'live' = 'replaying';
  const pending: Array<{ data: string; seq: number }> = [];

  const unsub = TerminalService.onOutput(sessionId, (data, seq) => {
    if (phase === 'replaying') {
      pending.push({ data, seq });
    } else if (seq > lastSeq) {
      write(data);
    }
  });

  void TerminalService.replay(sessionId)
    .then(({ data, lastSeq: ls }) => {
      lastSeq = ls;
      if (data) write(data);
      for (const c of pending) if (c.seq > lastSeq) write(c.data);
      pending.length = 0;
      phase = 'live';
    })
    .catch(() => {
      for (const c of pending) write(c.data);
      pending.length = 0;
      phase = 'live';
    });

  return unsub;
}
