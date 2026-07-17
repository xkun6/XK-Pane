import type { FoundationThemePreset } from '../types';

// 暗色主题：分层 slate 画布（非纯黑），长时间编码不刺眼。
// 选色参考 Tailwind slate + 现代 IDE：content → surface → elevated 三级抬升。
export const darkPreset: FoundationThemePreset = {
  name: 'foundation-dark',
  label: 'Dark',
  mode: 'dark',
  palette: {
    bg: {
      base: '#0a0c10',
      sidebar: '#0f1218',
      content: '#13171e',
      surface: '#1a1f28',
      elevated: '#232933',
      hover: 'rgba(255, 255, 255, 0.05)',
      active: 'rgba(96, 165, 250, 0.16)',
    },
    text: {
      primary: '#f1f5f9',
      secondary: '#94a3b8',
      muted: '#64748b',
    },
    divider: 'rgba(148, 163, 184, 0.12)',
    accent: '#60a5fa',
    accentHover: '#93c5fd',
    status: {
      danger: '#f87171',
      success: '#4ade80',
      warning: '#fbbf24',
    },
  },
};
