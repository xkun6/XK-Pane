// TerminalView 的 ViewModel：把一个后端 PTY 会话（sessionId）渲染成可交互 xterm。
//
// MVP 取自 cc-pane `TerminalView.tsx` 的核心 xterm 接线（其全量 1256 行含 provider/resume/
// SSH/WSL/popout/drag/reconnect/restore，均非 MVP）。本实现只做：
//   创建 Terminal + FitAddon → open 进 host → layoutScheduler 接 ResizeObserver →
//   term.onData → TerminalService.write；订阅 onOutput(sessionId)→term.write、onExit→提示+回调；
//   卸载时彻底回收。
// 刻意延后（标 TODO）：WebGL 渲染器控制器（性能增强，用 DOM 渲染器）、运行时切主题热更
//   （现于 mount 捕获主题色）、Windows ConPTY buildNumber 调优、写入流控、reconnect/restore、
//   wheel→方向键转换、drag fit 节流。
//
// MVVM：本 hook 不返回 JSX，只创建并返回 hostRef + 管理副作用；经 services/TerminalService
//   与后端通信，绝不直接 import @bindings（xterm 为第三方库，允许直接 import）。
import { useEffect, useRef } from 'react';
import { useTheme } from '@mui/material';
import { FitAddon } from '@xterm/addon-fit';
import { Terminal, type ITheme } from '@xterm/xterm';
import { TerminalService } from '@/services';
import {
  createTerminalLayoutScheduler,
  type TerminalLayoutScheduler,
} from '@/terminal/layoutScheduler';

const TERMINAL_FONT_FAMILY =
  'Consolas, "Courier New", "Microsoft YaHei Mono", "Noto Sans Mono CJK SC", "PingFang SC", monospace';
const TERMINAL_SCROLLBACK = 10000;
const RESIZE_DEBOUNCE_MS = 150;
const MIN_CONTAINER_DELTA = 5;

const noopLogger = (): void => {};

export interface UseTerminalViewArgs {
  /** 已绑定的后端 PTY 会话 id；null = 尚未启动（仅渲染空终端，等待回填）。 */
  sessionId: string | null;
  /** 是否当前活动面板：非活动时延后 fit，切回可见时 flush + 安全聚焦。 */
  active: boolean;
  /** 会话退出回调（exitCode）。 */
  onExit?: (exitCode: number) => void;
}

export interface TerminalViewModel {
  hostRef: React.RefObject<HTMLDivElement | null>;
}

export function useTerminalView({ sessionId, active, onExit }: UseTerminalViewArgs): TerminalViewModel {
  const theme = useTheme();
  const hostRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const schedulerRef = useRef<TerminalLayoutScheduler | null>(null);

  // 让 scheduler getter / 异步回调读到最新值（避免过期闭包）。
  const sessionIdRef = useRef<string | null>(sessionId);
  const activeRef = useRef<boolean>(active);
  const onExitRef = useRef<UseTerminalViewArgs['onExit']>(onExit);
  useEffect(() => {
    sessionIdRef.current = sessionId;
    activeRef.current = active;
    onExitRef.current = onExit;
  });

  // 仅在 host 就绪后挂载一次：创建 xterm + 调度器 + 输入/尺寸接线。
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const xtermTheme: ITheme = {
      background: theme.palette.foundation.bg.content,
      foreground: theme.palette.foundation.text.primary,
      cursor: theme.palette.foundation.accent,
      cursorAccent: theme.palette.foundation.bg.content,
      selectionBackground: theme.palette.foundation.bg.active,
    };

    const term = new Terminal({
      allowProposedApi: true,
      cursorBlink: true,
      fontSize: 14,
      fontFamily: TERMINAL_FONT_FAMILY,
      scrollback: TERMINAL_SCROLLBACK,
      theme: xtermTheme,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    termRef.current = term;
    fitRef.current = fit;

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

    // 首次布局：非活动 tab 会留 pending，切回可见时 flush。
    scheduler.schedule('initial.fit');

    return () => {
      onData.dispose();
      observer.disconnect();
      scheduler.dispose();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
      schedulerRef.current = null;
    };
    // 只在 mount 接线一次；theme 于此捕获（运行时切主题热更属 TODO）。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 订阅当前会话的输出 / 退出；会话变更时重订阅。
  useEffect(() => {
    if (!sessionId) return;
    const offOutput = TerminalService.onOutput(sessionId, (data) => {
      termRef.current?.write(data);
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
