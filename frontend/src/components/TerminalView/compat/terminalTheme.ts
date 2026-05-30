// 终端 ANSI 调色板 —— 供 xterm 渲染与 OSC 颜色查询应答共用同一组色。
//
// 说明（与「样式只取 theme.palette.foundation.*」铁律的边界）：这里的 16 色 ANSI 是
// **终端内容域**常量（终端模拟器的固有调色板），不是 MUI UI chrome，foundation 主题里
// 没有对应槽位；cc-pane 的 terminalTheme.ts 同样硬编码。前景 / 背景由调用方从 foundation
// 主题注入（buildTerminalPalette 的入参），保证终端底色跟随应用主题。
export interface TerminalThemePalette {
  foreground: string;
  background: string;
  black: string;
  red: string;
  green: string;
  yellow: string;
  blue: string;
  magenta: string;
  cyan: string;
  white: string;
  brightBlack: string;
  brightRed: string;
  brightGreen: string;
  brightYellow: string;
  brightBlue: string;
  brightMagenta: string;
  brightCyan: string;
  brightWhite: string;
}

// xterm 常见默认 ANSI 16 色（Tango 调色板）。
const ANSI16 = {
  black: '#2e3436',
  red: '#cc0000',
  green: '#4e9a06',
  yellow: '#c4a000',
  blue: '#3465a4',
  magenta: '#75507b',
  cyan: '#06989a',
  white: '#d3d7cf',
  brightBlack: '#555753',
  brightRed: '#ef2929',
  brightGreen: '#8ae234',
  brightYellow: '#fce94f',
  brightBlue: '#729fcf',
  brightMagenta: '#ad7fa8',
  brightCyan: '#34e2e2',
  brightWhite: '#eeeeec',
} as const;

// 用注入的前景 / 背景 + 固定 ANSI 16 色构造完整调色板。
export function buildTerminalPalette(foreground: string, background: string): TerminalThemePalette {
  return { foreground, background, ...ANSI16 };
}
