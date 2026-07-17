import type { FoundationThemePreset } from '../types';

// 黑曜：近 OLED 纯黑 + 紫罗兰强调，对比度高于普通暗色，适合夜间深工作。
// 与 dark 预设语义槽位完全一致，仅替换颜色值。
export const obsidianPreset: FoundationThemePreset = {
  name: 'foundation-obsidian',
  label: 'Obsidian',
  mode: 'dark',
  palette: {
    bg: {
      base: '#000000',
      sidebar: '#07070a',
      content: '#0b0b0f',
      surface: '#131318',
      elevated: '#1a1a22',
      hover: 'rgba(167, 139, 250, 0.08)',
      active: 'rgba(167, 139, 250, 0.18)',
    },
    text: {
      primary: '#f4f4f8',
      secondary: '#a1a1b2',
      muted: '#6b6b7b',
    },
    divider: 'rgba(255, 255, 255, 0.07)',
    accent: '#a78bfa',
    accentHover: '#c4b5fd',
    status: {
      danger: '#f87171',
      success: '#4ade80',
      warning: '#fbbf24',
    },
  },
};
