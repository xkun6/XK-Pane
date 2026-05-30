// OSC 颜色查询应答 —— 忠实移植 cc-pane web/components/panes/terminalOscColor.ts。
// TUI 会用 OSC 10/11 查询前景/背景色、OSC 4 查询调色板色（探测终端主题以适配显示）。
// 我们按 xterm 实际使用的调色板回应。ESC/ST 用常量避免转义歧义；strict 安全守门。
import { ESC, ST } from './esc';
import type { TerminalThemePalette } from './terminalTheme';

const ANSI_COLOR_KEYS = [
  'black',
  'red',
  'green',
  'yellow',
  'blue',
  'magenta',
  'cyan',
  'white',
  'brightBlack',
  'brightRed',
  'brightGreen',
  'brightYellow',
  'brightBlue',
  'brightMagenta',
  'brightCyan',
  'brightWhite',
] as const satisfies ReadonlyArray<keyof TerminalThemePalette>;

function formatOscRgbColor(hex: string): string | null {
  const match = /^#([0-9a-f]{6})$/i.exec(hex.trim());
  if (!match) return null;
  const hexBody = match[1];
  if (hexBody === undefined) return null;

  const [r, g, b] = [
    hexBody.slice(0, 2),
    hexBody.slice(2, 4),
    hexBody.slice(4, 6),
  ].map((part) => part.toLowerCase());

  return `rgb:${r}${r}/${g}${g}/${b}${b}`;
}

function buildOscReply(ident: number, color: string): string | null {
  const formatted = formatOscRgbColor(color);
  if (!formatted) return null;
  return `${ESC}]${ident};${formatted}${ST}`;
}

function buildPaletteReply(index: number, color: string): string | null {
  const formatted = formatOscRgbColor(color);
  if (!formatted) return null;
  return `${ESC}]4;${index};${formatted}${ST}`;
}

export function buildOscColorReply(
  ident: number,
  data: string,
  theme: TerminalThemePalette,
): string | null {
  const trimmed = data.trim();

  if (ident === 10 && trimmed === '?') {
    return buildOscReply(10, theme.foreground);
  }
  if (ident === 11 && trimmed === '?') {
    return buildOscReply(11, theme.background);
  }
  if (ident !== 4) {
    return null;
  }

  const [rawIndex, rawQuery] = trimmed.split(';', 2);
  if (rawQuery !== '?' || rawIndex === undefined) {
    return null;
  }

  const index = Number.parseInt(rawIndex, 10);
  if (!Number.isInteger(index) || index < 0 || index >= ANSI_COLOR_KEYS.length) {
    return null;
  }
  const key = ANSI_COLOR_KEYS[index];
  if (key === undefined) return null;

  return buildPaletteReply(index, theme[key]);
}
