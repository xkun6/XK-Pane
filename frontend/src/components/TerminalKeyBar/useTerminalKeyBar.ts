// TerminalKeyBar 的 ViewModel：返回移动端虚拟键栏的按键描述（序列 + 标签/图标标记）。
// 移植自终端控制需求：手机软键盘缺 Esc/Tab/Ctrl/方向键，而 claude/codex 这类 TUI 全靠它们
// 导航与取消（P7 M6 移动端虚拟键栏）。本 hook 不持翻译串（labelKey 给 View 调 t()），不返回 JSX。

// 各键写入 PTY 的原始字节序列。
export const KEY_SEQUENCES = {
  esc: '\x1b',
  tab: '\t',
  ctrlC: '\x03',
  up: '\x1b[A',
  down: '\x1b[B',
  left: '\x1b[D',
  right: '\x1b[C',
} as const;

export type KeyId = keyof typeof KEY_SEQUENCES;

// 方向键用 MUI 图标渲染（Icon 铁律：禁 Unicode 箭头当图标）；文本键用 labelKey 走 t()。
export type KeyKind = 'text' | 'arrow';

export interface KeyDescriptor {
  id: KeyId;
  seq: string;
  kind: KeyKind;
  /** kind==='text' 时的文案 i18n key；kind==='arrow' 时图标由 View 按 id 选。 */
  labelKey?: string;
  /** 无障碍标签 i18n key。 */
  ariaKey: string;
}

export interface UseTerminalKeyBar {
  keys: KeyDescriptor[];
}

export function useTerminalKeyBar(): UseTerminalKeyBar {
  const keys: KeyDescriptor[] = [
    { id: 'esc', seq: KEY_SEQUENCES.esc, kind: 'text', labelKey: 'terminalKeyBar.esc', ariaKey: 'terminalKeyBar.esc' },
    { id: 'tab', seq: KEY_SEQUENCES.tab, kind: 'text', labelKey: 'terminalKeyBar.tab', ariaKey: 'terminalKeyBar.tab' },
    { id: 'ctrlC', seq: KEY_SEQUENCES.ctrlC, kind: 'text', labelKey: 'terminalKeyBar.ctrlC', ariaKey: 'terminalKeyBar.ctrlC' },
    { id: 'left', seq: KEY_SEQUENCES.left, kind: 'arrow', ariaKey: 'terminalKeyBar.left' },
    { id: 'up', seq: KEY_SEQUENCES.up, kind: 'arrow', ariaKey: 'terminalKeyBar.up' },
    { id: 'down', seq: KEY_SEQUENCES.down, kind: 'arrow', ariaKey: 'terminalKeyBar.down' },
    { id: 'right', seq: KEY_SEQUENCES.right, kind: 'arrow', ariaKey: 'terminalKeyBar.right' },
  ];
  return { keys };
}
