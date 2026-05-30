// 终端键盘：粘贴快捷键判定 —— 忠实移植 cc-pane web/components/panes/terminalKeyboard.ts。
// Mac 用 Cmd+V、其余用 Ctrl+V；Alt 修饰一律不算粘贴。
type TerminalKeyboardEvent = Pick<
  KeyboardEvent,
  'type' | 'key' | 'ctrlKey' | 'metaKey' | 'altKey'
>;

export function isTerminalPasteShortcut(
  event: TerminalKeyboardEvent,
  isMac: boolean,
): boolean {
  if (event.type !== 'keydown') return false;
  if (event.altKey) return false;
  if (event.key !== 'v' && event.key !== 'V') return false;

  if (isMac) {
    return event.metaKey && !event.ctrlKey;
  }
  return event.ctrlKey && !event.metaKey;
}

// Ctrl/Cmd+C 复制快捷键判定（用于「有选区则复制、无选区透传 SIGINT」）。
export function isTerminalCopyShortcut(
  event: TerminalKeyboardEvent,
  isMac: boolean,
): boolean {
  if (event.type !== 'keydown') return false;
  if (event.altKey) return false;
  if (event.key !== 'c' && event.key !== 'C') return false;

  if (isMac) {
    return event.metaKey && !event.ctrlKey;
  }
  return event.ctrlKey && !event.metaKey;
}
