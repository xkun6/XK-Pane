// ScreenshotService：截图快捷键的前端封装（参照 cc-pane screenshotService）。
//
// 截图流程（热键 → 截屏 → 选区 → 存 PNG → 路径写剪贴板 + screenshot:saved 事件）全在
// 后端原生实现；前端仅负责改键（UpdateShortcut）、事件订阅与设置项持久化（preferences KV）。
import { Service as Binding } from '@bindings/xkpane/internal/services/screenshotsvc';
import { AppEvents } from '@/shared/events';
import { eventTransport, isDesktop } from '@/services/transport';

// preferences 键与默认值 —— 与后端 internal/services/screenshotsvc 常量严格一致。
export const SCREENSHOT_PREF_SHORTCUT = 'screenshot.shortcut';
export const SCREENSHOT_PREF_RETENTION_DAYS = 'screenshot.retentionDays';
export const SCREENSHOT_DEFAULT_SHORTCUT = 'Ctrl+Alt+S';
export const SCREENSHOT_DEFAULT_RETENTION_DAYS = 7;

// screenshot:saved 事件载荷（对齐 internal/events.ScreenshotSavedPayload）。
export interface ScreenshotSavedPayload {
  filePath: string;
  width: number;
  height: number;
}

export const ScreenshotService = {
  // 运行时改键：后端注销旧热键并注册新热键；键位冲突时 reject。注意后端语义是
  // 「旧键先注销」——失败时旧键也已失效，调用方需回滚注册或明确提示。
  // 仅注册，不持久化 —— 成功后由调用方经 PreferencesService 落库。
  async updateShortcut(shortcut: string): Promise<void> {
    await Binding.UpdateShortcut(shortcut);
  },

  // 订阅截图保存事件（解包后的 payload）。截图是桌面专属能力，viewer 端返回 no-op。
  onSaved(handler: (payload: ScreenshotSavedPayload) => void): () => void {
    if (!isDesktop) return () => {};
    return eventTransport.on<ScreenshotSavedPayload>(AppEvents.ScreenshotSaved, handler);
  },

  // 终端 Ctrl+V 文件/图片粘贴（对译 cc-pane terminalClipboard ②流）：paste 事件无文本时
  // 调用。CF_HDROP 返回文件路径列表；剪贴板只有位图时由后端落盘 screenshots 目录、返回该
  // PNG 路径；都没有返回空数组。仅 Windows 有内容，其余平台 / viewer 恒空。
  async clipboardPastePaths(): Promise<string[]> {
    return (await Binding.ClipboardPastePaths()) ?? [];
  },
} as const;
