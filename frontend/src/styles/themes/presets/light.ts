import type { FoundationThemePreset } from '../types';

// 明亮主题：软画布 + 白卡片层级（Linear / Cursor 系桌面工具感）。
// 设计目标：
//  - content 用微灰画布，surface 纯白卡片浮起，避免「整页一片白」
//  - sidebar / base 再降一级，导航与主区有清晰分层
//  - elevated 给输入框 / 次级控件可见底色
//  - accent 用现代亮蓝，active 用同色半透明
export const lightPreset: FoundationThemePreset = {
  name: 'foundation-light',
  label: 'Light',
  mode: 'light',
  palette: {
    bg: {
      base: '#eef0f4',
      sidebar: '#f3f4f8',
      content: '#f7f8fb',
      surface: '#ffffff',
      elevated: '#eef1f6',
      hover: 'rgba(15, 23, 42, 0.05)',
      active: 'rgba(59, 130, 246, 0.12)',
    },
    text: {
      primary: '#0f172a',
      secondary: '#475569',
      muted: '#94a3b8',
    },
    divider: 'rgba(15, 23, 42, 0.08)',
    accent: '#3b82f6',
    accentHover: '#2563eb',
    status: {
      danger: '#ef4444',
      success: '#22c55e',
      warning: '#f59e0b',
    },
  },
};
