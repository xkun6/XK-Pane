// CPR（Cursor Position Report）应答 —— 忠实移植 cc-pane web/components/panes/terminalCpr.ts。
// 交互式 TUI（codex / vim / less）会发 `CSI 6 n`(或 `CSI ? 6 n`) 查询光标位置，
// 期望终端回 `CSI row;col R`。cc-pane 显式接管以保证 codex 等握手正确。
// 按 noUncheckedIndexedAccess 加了 undefined 守门；ESC 用常量避免转义歧义。
import { ESC } from './esc';

export function buildCursorPositionReport(
  params: Array<number | number[]>,
  prefix: string | undefined,
  cursorX: number,
  cursorY: number,
): string | null {
  if (params.length !== 1) return null;

  const first = params[0];
  if (first === undefined) return null;
  const code = typeof first === 'number' ? first : first[0];
  if (code !== 6) return null;

  const row = Math.max(1, cursorY + 1);
  const col = Math.max(1, cursorX + 1);

  if (prefix === '?') {
    return `${ESC}[?${row};${col}R`;
  }
  if (!prefix) {
    return `${ESC}[${row};${col}R`;
  }

  return null;
}
