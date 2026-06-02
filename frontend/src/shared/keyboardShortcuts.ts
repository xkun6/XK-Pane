import { isMac } from './platform';

// 全局快捷键的展示元数据 + 按平台格式化（Sidebar tooltip 用）。
// 实际按键分发在 shared/hooks/useGlobalShortcuts.ts —— 二者约定保持一致。
//
// Mod = Windows/Linux 的 Ctrl、macOS 的 ⌘。终端（xterm）聚焦时所有全局键让行；
// Monaco 仅占用 Mod+S（保存），与此处不冲突。
export interface ShortcutHint {
  mods: ('mod' | 'shift')[];
  key: string; // 显示用键名：'G' / ',' / '1' …
}

const MOD_LABEL = isMac ? '⌘' : 'Ctrl';
const SHIFT_LABEL = isMac ? '⇧' : 'Shift';

// 组合键文本：mac 用紧凑符号（⌘⇧G），其余用 + 连接（Ctrl+Shift+G）。
export function formatShortcut(h: ShortcutHint): string {
  const parts: string[] = h.mods.map((m) => (m === 'mod' ? MOD_LABEL : SHIFT_LABEL));
  parts.push(h.key);
  return isMac ? parts.join('') : parts.join('+');
}

// 右侧抽屉开关的提示组合键（须与 useGlobalShortcuts 中的判定一致）。
export const PANEL_SHORTCUT: Record<'git' | 'history', ShortcutHint> = {
  git: { mods: ['mod', 'shift'], key: 'G' },
  history: { mods: ['mod', 'shift'], key: 'H' },
};

// 设置页的提示组合键（Mod+,，对齐 VSCode）。
export const SETTINGS_SHORTCUT: ShortcutHint = { mods: ['mod'], key: ',' };
