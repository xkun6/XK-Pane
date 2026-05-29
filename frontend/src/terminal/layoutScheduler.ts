// 终端布局调度器 —— 忠实移植 cc-pane `web/components/panes/terminalLayoutScheduler.ts`（逐字，仅本注释为新增）。
//
// 职责：把 xterm 的 fit / 后端 resize / 重绘 收敛成「双 rAF + 可选 delay 去抖」调度，
// 并在以下情况跳过：面板非活动、宿主未渲染（display:none / 0 尺寸 / 脱离文档）、容器抖动
// （minContainerDelta 阈值内）。仅当 cols/rows 实际变化且有 sessionId 时才回调 resizeBackend。
//
// 设计上全部依赖经 getter / callback 注入（getTerminal/getFitAddon/getHost/getSessionId/
// isActive/repaint/resizeBackend/logger），故可用 fake term/fit + jsdom 宿主单测（见同名 .test）。
// WebGL 渲染器控制器（cc-pane terminalRendererController）属性能增强，MVP 用 xterm DOM 渲染器，暂不移植。
import type { FitAddon } from '@xterm/addon-fit';
import type { Terminal } from '@xterm/xterm';

type LayoutLogger = (event: string, payload?: Record<string, unknown>) => void;

export interface TerminalContainerSize {
  width: number;
  height: number;
}

export interface TerminalLayoutRequestOptions {
  focusIfSafe?: boolean;
  delayMs?: number;
  containerSize?: TerminalContainerSize;
  minContainerDelta?: number;
  force?: boolean;
  onAfterLayout?: (term: Terminal) => void;
}

export interface TerminalLayoutScheduler {
  schedule: (reason: string, options?: TerminalLayoutRequestOptions) => void;
  flush: (reason: string, options?: TerminalLayoutRequestOptions) => Terminal | null;
  cancel: () => void;
  dispose: () => void;
  hasPendingLayout: () => boolean;
}

interface CreateTerminalLayoutSchedulerOptions {
  getTerminal: () => Terminal | null;
  getFitAddon: () => FitAddon | null;
  getHost: () => HTMLElement | null;
  getSessionId: () => string | null;
  isActive: () => boolean;
  repaint: (reason: string) => void;
  resizeBackend: (cols: number, rows: number) => void;
  logger: LayoutLogger;
}

export function isTerminalHostRenderable(host: HTMLElement | null): boolean {
  if (!host || !host.isConnected) return false;

  const style = window.getComputedStyle(host);
  if (style.display === 'none' || style.visibility === 'hidden') return false;

  const rect = host.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}

function requestFrame(callback: FrameRequestCallback): number {
  if (typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function') {
    return window.requestAnimationFrame(callback);
  }
  return window.setTimeout(() => callback(performance.now()), 0);
}

function cancelFrame(id: number): void {
  if (typeof window !== 'undefined' && typeof window.cancelAnimationFrame === 'function') {
    window.cancelAnimationFrame(id);
    return;
  }
  window.clearTimeout(id);
}

function shouldFocusTerminal(): boolean {
  const active = document.activeElement;
  if (!active) return true;
  return active.tagName !== 'INPUT' && active.tagName !== 'TEXTAREA';
}

export function createTerminalLayoutScheduler({
  getTerminal,
  getFitAddon,
  getHost,
  getSessionId,
  isActive,
  repaint,
  resizeBackend,
  logger,
}: CreateTerminalLayoutSchedulerOptions): TerminalLayoutScheduler {
  let rafId: number | null = null;
  let nestedRafId: number | null = null;
  let timerId: ReturnType<typeof setTimeout> | null = null;
  let disposed = false;
  let pendingReason: string | null = null;
  let lastSize: { cols: number; rows: number } | null = null;
  let lastContainerSize: TerminalContainerSize | null = null;

  const cancel = () => {
    if (timerId !== null) {
      clearTimeout(timerId);
      timerId = null;
    }
    if (rafId !== null) {
      cancelFrame(rafId);
      rafId = null;
    }
    if (nestedRafId !== null) {
      cancelFrame(nestedRafId);
      nestedRafId = null;
    }
  };

  const shouldSkipContainerDelta = (options: TerminalLayoutRequestOptions): boolean => {
    if (options.force || !options.containerSize || !options.minContainerDelta) return false;
    const size = options.containerSize;
    if (!lastContainerSize) {
      lastContainerSize = size;
      return false;
    }

    const deltaWidth = Math.abs(size.width - lastContainerSize.width);
    const deltaHeight = Math.abs(size.height - lastContainerSize.height);
    if (deltaWidth < options.minContainerDelta && deltaHeight < options.minContainerDelta) {
      logger('layout.skip.container-jitter', {
        width: size.width,
        height: size.height,
        deltaWidth,
        deltaHeight,
      });
      return true;
    }

    lastContainerSize = size;
    return false;
  };

  const applyLayout = (
    reason: string,
    options: TerminalLayoutRequestOptions = {},
  ): Terminal | null => {
    if (disposed) return null;
    if (shouldSkipContainerDelta(options)) return null;

    const term = getTerminal();
    const fitAddon = getFitAddon();
    const host = getHost();
    if (!term || !fitAddon || !host) return null;

    if (!isActive()) {
      pendingReason = reason;
      logger('layout.skip.inactive', { reason });
      return null;
    }

    const rect = host.getBoundingClientRect();
    if (!isTerminalHostRenderable(host)) {
      pendingReason = reason;
      logger('layout.skip.not-renderable', {
        reason,
        isConnected: host.isConnected,
        width: rect.width,
        height: rect.height,
      });
      return null;
    }

    try {
      fitAddon.fit();
    } catch (error) {
      logger('layout.fit.fail', {
        reason,
        error: error instanceof Error ? error.message : String(error),
      });
      return null;
    }

    repaint(reason);

    if (options.focusIfSafe && shouldFocusTerminal()) {
      term.focus();
    }

    const { cols, rows } = term;
    if (lastSize?.cols !== cols || lastSize?.rows !== rows) {
      lastSize = { cols, rows };
      if (getSessionId()) {
        resizeBackend(cols, rows);
      }
    }

    pendingReason = null;
    logger('layout.applied', {
      reason,
      cols,
      rows,
      width: rect.width,
      height: rect.height,
      sessionId: getSessionId(),
    });
    options.onAfterLayout?.(term);
    return term;
  };

  const schedule = (reason: string, options: TerminalLayoutRequestOptions = {}) => {
    if (disposed) return;
    cancel();

    const run = () => {
      rafId = requestFrame(() => {
        nestedRafId = requestFrame(() => {
          rafId = null;
          nestedRafId = null;
          applyLayout(reason, options);
        });
      });
    };

    if (options.delayMs && options.delayMs > 0) {
      timerId = setTimeout(() => {
        timerId = null;
        run();
      }, options.delayMs);
      return;
    }

    run();
  };

  return {
    schedule,
    flush: (reason, options) => {
      cancel();
      return applyLayout(reason, options);
    },
    cancel,
    dispose: () => {
      disposed = true;
      cancel();
      pendingReason = null;
      lastContainerSize = null;
      lastSize = null;
    },
    hasPendingLayout: () => pendingReason !== null,
  };
}
