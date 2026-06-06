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

// 终端色彩模式：跟随应用主题，决定选哪套 ANSI 16 色。
export type TerminalColorMode = 'light' | 'dark';

// 暗底 ANSI 16 色（Tango 调色板）—— xterm 常见默认，亮色分量偏高，配深色背景。
const ANSI16_DARK = {
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

// 亮底 ANSI 16 色 —— 专为纯白背景（foundation-light 的 bg.content=#ffffff）调过，每色对白底 WCAG 对比
// 均 ≥4.5:1（正文可读门槛）。不能直接照搬 VS Code「Light+」那套：它配自家非纯白编辑器底、且多数色当背景
// 用，绿/亮绿/亮黄/亮白在纯白上当前景文字时只有 ~2:1，几乎发虚（agent CLI 把 diff 新增行 / 成功文本染绿，
// 正是看不清的症结）。绿/黄因人眼亮度权重高必须显著压暗才能达标；亮色与普通色保留可辨色相差异。
const ANSI16_LIGHT = {
  black: '#000000',
  red: '#cd3131',
  green: '#008000',
  yellow: '#8a6d00',
  blue: '#0451a5',
  magenta: '#bc05bc',
  cyan: '#00788c',
  white: '#555555',
  brightBlack: '#6c6c6c',
  brightRed: '#d32f2f',
  brightGreen: '#008a00',
  brightYellow: '#a06a00',
  brightBlue: '#1565c0',
  brightMagenta: '#c2186a',
  brightCyan: '#0e7490',
  brightWhite: '#767676',
} as const;

// 用注入的前景 / 背景 + 跟随主题模式的 ANSI 16 色构造完整调色板。
export function buildTerminalPalette(
  foreground: string,
  background: string,
  mode: TerminalColorMode = 'dark',
): TerminalThemePalette {
  const ansi = mode === 'light' ? ANSI16_LIGHT : ANSI16_DARK;
  return { foreground, background, ...ansi };
}
