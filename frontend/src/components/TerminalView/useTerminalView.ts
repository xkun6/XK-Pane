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
//   - 剪贴板文件/图片粘贴（cc-pane terminalClipboard ②流）：paste 无文本时问后端换路径再 term.paste
// 仍刻意延后：内层分屏、reconnect/restore、wheel→方向键。
//
// MVVM：本 hook 不返回 JSX，只创建并返回 hostRef + 管理副作用；经 services/TerminalService
//   与后端通信，绝不直接 import @bindings（xterm 为第三方库，允许直接 import）。
import { useEffect, useRef } from 'react';
import { useTheme, type Theme } from '@mui/material';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { Terminal, type ITheme } from '@xterm/xterm';
import { ScreenshotService, TerminalService, attachSessionOutput } from '@/services';
import { isDesktop, lifecycleRole } from '@/services/transport';
import { isMac, isWindows } from '@/shared/platform';
import {
  createTerminalLayoutScheduler,
  isTerminalHostRenderable,
  type TerminalLayoutScheduler,
} from '@/terminal/layoutScheduler';
import { recordTerminalSize } from '@/terminal/terminalSize';
import { buildCursorPositionReport } from './compat/terminalCpr';
import {
  buildPrimaryDeviceAttributesReport,
  buildKittyKeyboardProtocolReport,
} from './compat/terminalCapabilityReports';
import { buildOscColorReply } from './compat/terminalOscColor';
import { buildTerminalPalette, type TerminalThemePalette } from './compat/terminalTheme';
import {
  shouldKeepCliOutputInNormalBuffer,
  stripAlternateBufferSequences,
} from './compat/terminalBufferMode';
import { formatPastedPaths } from './compat/terminalClipboardPaste';
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

// 把当前 MUI 主题映射成 xterm 主题 + 配套调色板（OSC 颜色应答与渲染共用同一组色）。
// 前景/背景/光标/选区取 foundation 语义槽位，ANSI 16 色按主题明暗选表（见 terminalTheme.ts）。
function buildXtermTheme(theme: Theme): { xtermTheme: ITheme; palette: TerminalThemePalette } {
  const palette = buildTerminalPalette(
    theme.palette.foundation.text.primary,
    theme.palette.foundation.bg.content,
    theme.palette.mode,
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
  return { xtermTheme, palette };
}

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
  // 当前终端调色板：OSC 颜色应答读它（运行时切主题后由热更新 effect 刷新），保证应答的 fg/bg/ANSI 是最新色。
  const paletteRef = useRef<TerminalThemePalette | null>(null);
  // 主题热更新是否已越过首帧：挂载 effect 已用初始 theme 设过 xterm 主题，热更新 effect 首跑应跳过（见下）。
  const themeHotSyncReadyRef = useRef(false);

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

    // 终端调色板 + xterm 主题：取自当前 foundation 主题（前景/背景/ANSI 随明暗切换）。palette 存入 ref
    // 供 OSC 颜色应答读取；运行时切主题由下方独立 effect 热更新（挂载时这里捕获的只是初始值）。
    const { xtermTheme, palette: initialPalette } = buildXtermTheme(theme);
    paletteRef.current = initialPalette;

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

    // 趁本组件（TerminalView）的 mount effect 先于 TerminalContext 的 reconcile effect 执行（React 子
    // effect 先于父）：open 后立即同步 fit 一次并记录真实列宽，让紧随其后的「创建会话」(含 resume) 直接
    // 拿到本终端容器自己的真实列宽作 PTY 初始尺寸，无需回退默认 80 —— reclaude --resume 从一开始就按对的
    // 列宽重放历史，根除冷启动首个会话的列宽不匹配。host 此刻已挂载、布局完成（commit 后），尺寸真实；隐藏
    // tab（尺寸 0）由 renderable 检查跳过，由已有终端的 lastTerminalSize 兜底。字体已 <link preload>，
    // 此刻大概率就绪，cell 宽准。
    if (isTerminalHostRenderable(host)) {
      try {
        fit.fit();
        recordTerminalSize(term.cols, term.rows);
      } catch {
        // host 尺寸异常时跳过；scheduler 的 initial.fit 会补一次。
      }
    }

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
    // OSC 颜色应答：读 paletteRef（运行时切主题后是最新色）；palette 未就绪时放行不应答。
    const oscReply = (ident: 4 | 10 | 11, data: string): boolean => {
      const p = paletteRef.current;
      return p ? reply(buildOscColorReply(ident, data, p)) : false;
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
      term.parser.registerOscHandler(4, (data) => oscReply(4, data)),
      term.parser.registerOscHandler(10, (data) => oscReply(10, data)),
      term.parser.registerOscHandler(11, (data) => oscReply(11, data)),
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
        // 记录真实 fit 列宽，供后续新建 / resume 会话作 PTY 初始尺寸（见 terminalSize.ts），避免
        // reclaude --resume 按后端默认列宽重放历史而错位。
        recordTerminalSize(cols, rows);
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

    // 文件/图片粘贴（对译 cc-pane terminalClipboard ②流）：剪贴板是文件对象（Explorer 复制
    // 的文件、XK 截图的 CF_HDROP）或纯位图（截图工具「复制图像」）时，浏览器 paste 事件的
    // text/plain 为空 → xterm 默认什么都不写（仅 bracketed 模式下发空括号对）。capture 阶段
    // 截获后问后端：文件取路径 / 位图落盘 screenshots 目录换路径，再经 term.paste() 写入 ——
    // xterm 会按应用是否开启 bracketed-paste 自动包 \x1b[200~（claude/codex 据此识别图片路径
    // 为附件）。有文本时不拦，照走 xterm 原生单份粘贴（见上方 Ctrl/Cmd+V 注释）。
    // 仅 Windows 桌面端：viewer 没有该 binding；macOS/Linux 后端恒空，且必须放行原生空粘贴
    // —— claude 收到空 bracketed 括号对会走自己的取图管线（osascript/xclip），拦了反而把
    // 改动前可用的图片粘贴弄失效（对抗审查确证的回归，勿改回只判 isDesktop）。
    const onHostPaste = (event: ClipboardEvent) => {
      if (!isDesktop || !isWindows) return;
      if (event.clipboardData?.getData('text/plain')) return;
      event.preventDefault();
      event.stopPropagation();
      void ScreenshotService.clipboardPastePaths()
        .then((paths) => {
          if (paths.length > 0) termRef.current?.paste(formatPastedPaths(paths));
        })
        .catch(() => {});
    };
    host.addEventListener('paste', onHostPaste, true);

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
      host.removeEventListener('paste', onHostPaste, true);
      host.removeEventListener('pointerup', focusOnTap);
      observer.disconnect();
      scheduler.dispose();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
      schedulerRef.current = null;
      flowRef.current = null;
    };
    // 只在 mount 接线一次；theme 仅用于构造初始 xterm 主题，运行时切主题由下方独立 effect 热更新。
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

  // 运行时切主题热更新：foundation 主题变化（显式切预设、或「跟随系统」明暗翻转）时，重建 xterm 主题 +
  // 调色板并应用到已挂载终端。term.options.theme 经 xterm onSpecificOptionChange('theme') → onChangeColors
  // 通知渲染器（含 WebGL addon 的 _refreshCharAtlas）整屏重新着色。没有这步，xterm 主题会停在挂载时捕获的
  // 值，切到暗色后终端仍是亮色、且亮底 ANSI 调色板不跟随明暗 —— 即本次修复的两个症状。
  useEffect(() => {
    // 跳过首帧：挂载 effect 已用同一 theme 经 new Terminal({theme}) 设过初始主题，首帧再赋值只会触发一次
    // 同色的图集重建（无害但白费）。只有主题真正变化（首帧之后）才热更新。
    if (!themeHotSyncReadyRef.current) {
      themeHotSyncReadyRef.current = true;
      return;
    }
    const term = termRef.current;
    if (!term) return;
    const { xtermTheme, palette } = buildXtermTheme(theme);
    paletteRef.current = palette;
    term.options.theme = xtermTheme;
  }, [
    theme.palette.mode,
    theme.palette.foundation.text.primary,
    theme.palette.foundation.bg.content,
    theme.palette.foundation.accent,
    theme.palette.foundation.bg.active,
  ]);

  return { hostRef };
}
