import { describe, expect, it } from 'vitest';
import { buildTerminalPalette } from './terminalTheme';

// 把 #rrggbb 转成 0–1 相对亮度（WCAG 相对亮度，用于「亮底不可见」回归断言）。
function relativeLuminance(hex: string): number {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m || m[1] === undefined) throw new Error(`bad hex: ${hex}`);
  const body = m[1];
  const chan = (i: number): number => {
    const v = parseInt(body.slice(i, i + 2), 16) / 255;
    return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * chan(0) + 0.7152 * chan(2) + 0.0722 * chan(4);
}

describe('buildTerminalPalette', () => {
  it('前景/背景透传，与 ANSI 表无关', () => {
    const p = buildTerminalPalette('#112233', '#ffffff', 'light');
    expect(p.foreground).toBe('#112233');
    expect(p.background).toBe('#ffffff');
  });

  it('默认 mode 为 dark（向后兼容旧调用）', () => {
    const def = buildTerminalPalette('#fff', '#000');
    const dark = buildTerminalPalette('#fff', '#000', 'dark');
    expect(def.brightWhite).toBe(dark.brightWhite);
  });

  it('light 与 dark 取不同 ANSI 表', () => {
    const light = buildTerminalPalette('#000', '#fff', 'light');
    const dark = buildTerminalPalette('#fff', '#000', 'dark');
    expect(light.brightWhite).not.toBe(dark.brightWhite);
    expect(light.brightYellow).not.toBe(dark.brightYellow);
  });

  it('亮色主题下，所有 ANSI 色对白底达到 WCAG 正文可读门槛 (≥4.5:1)', () => {
    const light = buildTerminalPalette('#000', '#ffffff', 'light');
    const whiteLum = relativeLuminance('#ffffff');
    const keys = [
      'black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white',
      'brightBlack', 'brightRed', 'brightGreen', 'brightYellow',
      'brightBlue', 'brightMagenta', 'brightCyan', 'brightWhite',
    ] as const;
    for (const k of keys) {
      const contrast = (whiteLum + 0.05) / (relativeLuminance(light[k]) + 0.05);
      // 白底上每个 ANSI 色都需 ≥4.5:1（正文可读）。门槛刻意从严：VS Code Light+ 的绿/亮绿/亮黄/亮白
      // 在纯白上只有 ~2:1，正是 bug #2「亮色看不清」的根因，必须杜绝其再次混入。
      expect(contrast, `${k}=${light[k]} 对白底对比不足`).toBeGreaterThanOrEqual(4.5);
    }
  });
});
