// TerminalView 的 ViewModel：把一个后端 PTY 会话（sessionId）渲染成可交互 xterm。
//
// 在 cc-pane MVP 接线（创建 Terminal + FitAddon → open → layoutScheduler → onData→write；
// 订阅 onOutput→write、onExit→提示）基础上，补齐 cc-pane 列为「不可省」的 TUI 兼容层
// （让 Claude/Codex 这类 agent CLI 握手正确、不渲染错乱）：
//   - CPR 光标查询应答（CSI 6 n / CSI ? 6 n）
//   - Primary DA 应答（CSI c）+ Kitty 键盘协议应答（CSI ? u）—— codex 启动握手必需
//   - OSC 4/10/11 颜色查询应答
//   - 写入流控（Windows 高吞吐防卡）
//   - agent CLI 输出剥离 alternate-buffer 切屏序列（输出留 normal buffer，滚轮滚历史）
//   - Ctrl/Cmd+V 粘贴（拦掉 xterm 默认 ^V，交浏览器原生 paste 写一份）/ 有选区时 Ctrl/Cmd+C 复制（无选区透传 SIGINT）
// 仍刻意延后：WebGL 渲染器、运行时切主题热更、内层分屏、reconnect/restore、wheel→方向键、
//   剪贴板文件路径/图片粘贴（Tauri 专有，依赖未移植能力）。
//
// MVVM：本 hook 不返回 JSX，只创建并返回 hostRef + 管理副作用；经 services/TerminalService
//   与后端通信，绝不直接 import @bindings（xterm 为第三方库，允许直接 import）。
import { useEffect, useRef } from 'react';
import { useTheme } from '@mui/material';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { Terminal, type ITheme } from '@xterm/xterm';
import { TerminalService, attachSessionOutput } from '@/services';
import { lifecycleRole } from '@/services/transport';
import { isMac, isWindows } from '@/shared/platform';
import {
  createTerminalLayoutScheduler,
  type TerminalLayoutScheduler,
} from '@/terminal/layoutScheduler';
import { buildCursorPositionReport } from './compat/terminalCpr';
import {
  buildPrimaryDeviceAttributesReport,
  buildKittyKeyboardProtocolReport,
} from './compat/terminalCapabilityReports';
import { buildOscColorReply } from './compat/terminalOscColor';
import { buildTerminalPalette } from './compat/terminalTheme';
import {
  shouldKeepCliOutputInNormalBuffer,
  stripAlternateBufferSequences,
} from './compat/terminalBufferMode';
import { isTerminalCopyShortcut, isTerminalPasteShortcut } from './compat/terminalKeyboard';
import {
  createTerminalWriteFlowControl,
  type TerminalWriteFlowControl,
} from './compat/terminalWriteFlowControl';

// 拉丁 / 数字 / 符号 / box-drawing 用打包的 JetBrains Mono —— 移动端 WebView 缺 Consolas 等桌面
// 等宽字体时不再 fallback 到比例字体（那会让字符被等宽 cell 拉开）。CJK 字形回退到系统等宽全角
// 字体（配合 unicode11 保证 2 宽），ui-monospace / monospace 兜底。
const TERMINAL_FONT_FAMILY =
  '"JetBrains Mono", ui-monospace, "Noto Sans Mono CJK SC", "Microsoft YaHei Mono", "PingFang SC", monospace';
const TERMINAL_SCROLLBACK = 10000;
const RESIZE_DEBOUNCE_MS = 150;
const MIN_CONTAINER_DELTA = 5;

const noopLogger = (): void => {};

export interface UseTerminalViewArgs {
  /** 已绑定的后端 PTY 会话 id；null = 尚未启动（仅渲染空终端，等待回填）。 */
  sessionId: string | null;
  /** 是否当前活动面板：非活动时延后 fit，切回可见时 flush + 安全聚焦。 */
  active: boolean;
  /** 该会话的 CLI 工具 id（决定是否对 agent CLI 剥离 alternate-buffer 序列）。 */
  cliTool?: string;
  /** 会话退出回调（exitCode）。 */
  onExit?: (exitCode: number) => void;
}

export interface TerminalViewModel {
  hostRef: React.RefObject<HTMLDivElement | null>;
}

export function useTerminalView({
  sessionId,
  active,
  cliTool,
  onExit,
}: UseTerminalViewArgs): TerminalViewModel {
  const theme = useTheme();
  const hostRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const schedulerRef = useRef<TerminalLayoutScheduler | null>(null);
  const flowRef = useRef<TerminalWriteFlowControl | null>(null);

  // 让 scheduler getter / 异步回调读到最新值（避免过期闭包）。
  const sessionIdRef = useRef<string | null>(sessionId);
  const activeRef = useRef<boolean>(active);
  const cliToolRef = useRef<string | undefined>(cliTool);
  const onExitRef = useRef<UseTerminalViewArgs['onExit']>(onExit);
  useEffect(() => {
    sessionIdRef.current = sessionId;
    activeRef.current = active;
    cliToolRef.current = cliTool;
    onExitRef.current = onExit;
  });

  // 仅在 host 就绪后挂载一次：创建 xterm + 调度器 + 输入/尺寸接线 + TUI 兼容层。
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    // 终端调色板：前景/背景取 foundation 主题，ANSI 16 色取终端域常量（见 terminalTheme.ts）。
    const palette = buildTerminalPalette(
      theme.palette.foundation.text.primary,
      theme.palette.foundation.bg.content,
    );
    const xtermTheme: ITheme = {
      background: palette.background,
      foreground: palette.foreground,
      cursor: theme.palette.foundation.accent,
      cursorAccent: palette.background,
      selectionBackground: theme.palette.foundation.bg.active,
      black: palette.black,
      red: palette.red,
      green: palette.green,
      yellow: palette.yellow,
      blue: palette.blue,
      magenta: palette.magenta,
      cyan: palette.cyan,
      white: palette.white,
      brightBlack: palette.brightBlack,
      brightRed: palette.brightRed,
      brightGreen: palette.brightGreen,
      brightYellow: palette.brightYellow,
      brightBlue: palette.brightBlue,
      brightMagenta: palette.brightMagenta,
      brightCyan: palette.brightCyan,
      brightWhite: palette.brightWhite,
    };

    const term = new Terminal({
      allowProposedApi: true,
      cursorBlink: true,
      fontSize: 14,
      fontFamily: TERMINAL_FONT_FAMILY,
      scrollback: TERMINAL_SCROLLBACK,
      theme: xtermTheme,
    });
    // unicode11：用 Unicode 11 宽度表算 CJK / emoji 的列宽（xterm 默认旧表会把全角字符当半角 →
    // 中文光标 / 字符水平错位）。必须在 open 前 load 并激活。
    const unicode11 = new Unicode11Addon();
    term.loadAddon(unicode11);
    term.unicode.activeVersion = '11';

    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);

    // WebGL 渲染器：整屏 GPU canvas 取代 DOM 逐行 div —— 消除移动 WebView 上行间合成接缝（横线），
    // 兼带高吞吐渲染。必须在 open 之后 load（需 canvas 上下文）。旧 WebView 不支持 WebGL 或运行时
    // 上下文丢失时，dispose 让 xterm 自动回退到 DOM 渲染器（功能不缺，仅失去接缝修复 / 性能增益）。
    let webgl: WebglAddon | null = null;
    try {
      webgl = new WebglAddon();
      webgl.onContextLoss(() => {
        webgl?.dispose();
        webgl = null;
      });
      term.loadAddon(webgl);
    } catch {
      webgl = null;
    }

    termRef.current = term;
    fitRef.current = fit;

    // ── TUI 兼容层：注册 CSI/OSC 应答处理器，回应经 PTY 写回 ──
    // reply: 有应答则写回 PTY 并返回 true（抑制 xterm 默认处理，避免重复应答）；否则放行。
    // 多端去重（P7 M6）：仅 owner（桌面）回写 TUI 自动应答（CPR/DA/OSC/Kitty）。viewer（Web）
    // 仍返回 true 抑制本端默认处理，但**不**回写 —— 否则桌面 + 手机同时应答，CLI 收到双份应答会乱。
    // viewer 的普通键入（onData / 虚拟键栏）不受影响，照常写回。
    const reply = (payload: string | null): boolean => {
      if (!payload) return false;
      if (lifecycleRole !== 'owner') return true;
      const sid = sessionIdRef.current;
      if (sid) void TerminalService.write(sid, payload).catch(() => {});
      return true;
    };
    const disposables = [
      term.parser.registerCsiHandler({ final: 'n' }, (params) =>
        reply(
          buildCursorPositionReport(
            params,
            undefined,
            term.buffer.active.cursorX,
            term.buffer.active.cursorY,
          ),
        ),
      ),
      term.parser.registerCsiHandler({ prefix: '?', final: 'n' }, (params) =>
        reply(
          buildCursorPositionReport(
            params,
            '?',
            term.buffer.active.cursorX,
            term.buffer.active.cursorY,
          ),
        ),
      ),
      term.parser.registerCsiHandler({ final: 'c' }, (params) =>
        reply(buildPrimaryDeviceAttributesReport(params, undefined)),
      ),
      term.parser.registerCsiHandler({ prefix: '?', final: 'u' }, (params) =>
        reply(buildKittyKeyboardProtocolReport(params, '?')),
      ),
      term.parser.registerOscHandler(4, (data) => reply(buildOscColorReply(4, data, palette))),
      term.parser.registerOscHandler(10, (data) => reply(buildOscColorReply(10, data, palette))),
      term.parser.registerOscHandler(11, (data) => reply(buildOscColorReply(11, data, palette))),
    ];

    // 写入流控：Windows 上启用 watermark（高吞吐输出防卡 webview 主线程）。
    const flow = createTerminalWriteFlowControl(
      { write: (data, cb) => termRef.current?.write(data, cb) },
      { enabled: isWindows },
    );
    flowRef.current = flow;

    // 键盘：
    //   Ctrl/Cmd+V —— 仅 return false：拦掉 xterm 默认把它编码成 ^V（\x16）的行为，但「不」
    //     preventDefault，于是浏览器原生 paste 事件照常触发 → xterm 自带 paste 监听写入一份
    //     （自动套 bracketed-paste 标记 \x1b[200~…\x1b[201~，对 claude/codex 多行粘贴更正确）。
    //     切勿在此再手动读剪贴板写 PTY —— 原生 paste 已写一份，手动再写就会双份（"你好"→"你好你好"）。
    //   Ctrl/Cmd+C —— 有选区则复制、无选区 return true 放行透传 SIGINT。
    term.attachCustomKeyEventHandler((event) => {
      if (isTerminalPasteShortcut(event, isMac)) return false;
      if (isTerminalCopyShortcut(event, isMac)) {
        const sel = termRef.current?.getSelection();
        if (sel && navigator.clipboard?.writeText) {
          void navigator.clipboard.writeText(sel).catch(() => {});
          return false;
        }
      }
      return true;
    });

    const scheduler = createTerminalLayoutScheduler({
      getTerminal: () => termRef.current,
      getFitAddon: () => fitRef.current,
      getHost: () => hostRef.current,
      getSessionId: () => sessionIdRef.current,
      isActive: () => activeRef.current,
      repaint: () => {
        requestAnimationFrame(() => {
          const t = termRef.current;
          if (!t) return;
          try {
            t.refresh(0, Math.max(0, t.rows - 1));
          } catch {
            // 重绘失败不应阻断后续布局。
          }
        });
      },
      resizeBackend: (cols, rows) => {
        const sid = sessionIdRef.current;
        if (sid) void TerminalService.resize(sid, cols, rows).catch(() => {});
      },
      logger: noopLogger,
    });
    schedulerRef.current = scheduler;

    const onData = term.onData((data) => {
      const sid = sessionIdRef.current;
      if (sid) void TerminalService.write(sid, data).catch(() => {});
    });

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (!entry) return;
      const { width, height } = entry.contentRect;
      scheduler.schedule('resize-observer.fit', {
        delayMs: RESIZE_DEBOUNCE_MS,
        containerSize: { width, height },
        minContainerDelta: MIN_CONTAINER_DELTA,
      });
    });
    observer.observe(host);

    // 移动端：点触终端区域即聚焦 xterm 隐藏 textarea，唤起软键盘（P7 M6）。桌面点击 xterm 本就
    // 聚焦，此举对触摸更可靠；不抢已聚焦态，无副作用。
    const focusOnTap = () => termRef.current?.focus();
    host.addEventListener('pointerup', focusOnTap);

    // 首次布局：非活动 tab 会留 pending，切回可见时 flush。
    scheduler.schedule('initial.fit');

    // @font-face 的 JetBrains Mono 是异步加载（font-display:swap）。上面 WebGL loadAddon 时已按
    // 「当时的字体」把字形栅格化进纹理图集、测 cell 尺寸 —— 若此刻字体尚未就绪，烘焙的是 fallback
    // 字形且 WebGL 不会自动重建（DOM 渲染器会随 swap 自动重排，故旧版桌面无此症，加 WebGL 后 resume
    // 满屏错位）。字体就绪后重设 fontFamily 触发 xterm 重测 cell + 渲染器重建图集，再强制 fit 让列数 /
    // 后端 PTY 宽度跟上变化后的 cell。document.fonts.load 主动拉取并等待（不靠按需加载 —— WebGL canvas
    // 不是 DOM 文本，不会触发 @font-face 按需加载）。chrome 验证：fallback 初始化的 WebGL 切 fontFamily
    // 后字形确实重建为 JetBrains Mono。
    if (typeof document !== 'undefined' && document.fonts?.load) {
      void document.fonts
        .load('14px "JetBrains Mono"')
        .then(() => {
          const t = termRef.current;
          if (!t) return;
          t.options.fontFamily = TERMINAL_FONT_FAMILY;
          schedulerRef.current?.schedule('font-ready.fit', { force: true });
        })
        .catch(() => {});
    }

    return () => {
      onData.dispose();
      for (const d of disposables) d.dispose();
      host.removeEventListener('pointerup', focusOnTap);
      observer.disconnect();
      scheduler.dispose();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
      schedulerRef.current = null;
      flowRef.current = null;
    };
    // 只在 mount 接线一次；theme 于此捕获（运行时切主题热更属 TODO）。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 订阅当前会话的输出 / 退出；会话变更时重订阅。
  useEffect(() => {
    if (!sessionId) return;
    // attachSessionOutput（P7 M4）：先重放原始 ANSI 历史画面再切 live（不漏不重），让中途
    // attach 到已运行会话（如桌面手点开的、Web 端点击「运行中」attach）能立即看到完整画面。
    // 桌面 replay 暂返回空 → 等价于纯 live（同步订阅 + 缓冲 catch-up，零回归）。处理逻辑（agent CLI
    // 切屏序列剥离 + 写入流控）对 replay 与 live 同样适用，故包在 write 回调里。
    const offOutput = attachSessionOutput(sessionId, (data) => {
      const tool = cliToolRef.current;
      const processed =
        tool && shouldKeepCliOutputInNormalBuffer(tool)
          ? stripAlternateBufferSequences(data)
          : data;
      const flow = flowRef.current;
      if (flow) void flow.write(processed);
      else termRef.current?.write(processed);
    });
    const offExit = TerminalService.onExit(sessionId, ({ exitCode }) => {
      termRef.current?.writeln(`\r\n\x1b[33mProcess exited with code ${exitCode}\x1b[0m`);
      onExitRef.current?.(exitCode);
    });
    // 会话刚绑定：强制 flush 一次，让后端拿到当前真实 cols/rows。
    schedulerRef.current?.schedule('session.attach', { force: true });
    return () => {
      offOutput();
      offExit();
    };
  }, [sessionId]);

  // 从隐藏切回可见：flush 待定布局 + 安全聚焦。
  useEffect(() => {
    if (active) {
      schedulerRef.current?.flush('active', { focusIfSafe: true });
    }
  }, [active]);

  return { hostRef };
}
