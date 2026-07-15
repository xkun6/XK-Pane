// ClipboardService：系统剪贴板读写门面，包 @wailsio/runtime 一等 Clipboard runtime。
//
// 钉死走 Wails runtime（非 navigator.clipboard.readText）——webview 下 readText 受聚焦/权限策略
// 常被拒；Wails Clipboard 由原生侧实现，稳。Web（viewer）端无桌面 runtime，readText 返空串。
import { Clipboard } from '@wailsio/runtime';
import { isDesktop } from '@/services/transport';

export const ClipboardService = {
  // 写剪贴板文本（桌面端走 Wails 原生 Clipboard，避开 WebView Clipboard API 权限/焦点限制）。
  async writeText(text: string): Promise<boolean> {
    if (!isDesktop) return false;
    await Clipboard.SetText(text);
    return true;
  },

  // 读剪贴板文本（无文本或非桌面端返回空串）。
  async readText(): Promise<string> {
    if (!isDesktop) return '';
    return (await Clipboard.Text()) ?? '';
  },
} as const;
