// 终端键盘：粘贴 / 复制快捷键判定 —— 移植自 cc-pane web/components/panes/terminalKeyboard.ts。
// Mac 用 Cmd+V/C、其余用 Ctrl+V/C；Alt 修饰一律不算。
type TerminalKeyboardEvent = Pick<
  KeyboardEvent,
  'type' | 'key' | 'ctrlKey' | 'metaKey' | 'altKey'
>;

// Ctrl/Cmd+V 粘贴快捷键判定。命中时调用方 return false：拦掉 xterm 默认的 ^V（\x16），
// 但不 preventDefault —— 让浏览器原生 paste 接管（单份 + 自动 bracketed-paste，见 useTerminalView.ts）。
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
