import type { SxProps, Theme } from '@mui/material';

// Web 访问子页面专属样式：配对二维码卡片 + URL 行 + 提示。
export const webAccessStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    card: {
      display: 'flex',
      flexDirection: 'column',
      gap: 2,
      p: 2.5,
      borderRadius: 1.5,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
    },
    muted: { fontSize: 13, color: fp.text.muted, lineHeight: 1.6 },

    // 二维码：固定白底（黑模块在任何主题下都可靠扫描）。
    qrSection: { display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1 },
    qrBox: { backgroundColor: '#fff', p: 1.5, borderRadius: 1, lineHeight: 0 },
    qrHint: { fontSize: 12, color: fp.text.muted, textAlign: 'center' },

    // URL 行
    urlRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      flexWrap: 'wrap',
      p: 1.25,
      borderRadius: 1,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
    },
    urlText: {
      flex: 1,
      minWidth: 0,
      wordBreak: 'break-all',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 12,
      color: fp.text.secondary,
    },
    urlTextPrimary: {
      flex: 1,
      minWidth: 0,
      wordBreak: 'break-all',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 13,
      color: fp.text.primary,
      fontWeight: 600,
    },
    copyBtn: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 0.5,
      flexShrink: 0,
      fontSize: 12,
      px: 1.5,
      py: 0.75,
      borderRadius: 1,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
      color: fp.text.primary,
      cursor: 'pointer',
      '&:hover': { backgroundColor: fp.bg.hover },
    },
    altBlock: { display: 'flex', flexDirection: 'column', gap: 0.75 },

    // 端口等元信息
    metaRow: { display: 'flex', alignItems: 'center', gap: 2 },
    metaLabel: {
      fontSize: 12,
      color: fp.text.muted,
      letterSpacing: 0.4,
      textTransform: 'uppercase',
      fontWeight: 600,
      width: 120,
      flexShrink: 0,
    },
    metaValue: {
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 13,
      color: fp.text.primary,
    },

    // 防火墙提示 / 密钥警告
    noteBox: {
      display: 'flex',
      alignItems: 'flex-start',
      gap: 1,
      p: 1.5,
      borderRadius: 1,
      backgroundColor: fp.bg.elevated,
      color: fp.text.secondary,
    },
    noteText: { fontSize: 12, lineHeight: 1.6, color: fp.text.secondary },
    warnText: { fontSize: 12, color: fp.status.warning, lineHeight: 1.6 },

    // 错误态
    errorRow: { display: 'flex', alignItems: 'center', gap: 1, color: fp.status.danger },
    errorText: { fontSize: 13, color: fp.status.danger },

    secondaryBtn: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 0.75,
      alignSelf: 'flex-start',
      fontSize: 13,
      px: 2,
      py: 1,
      borderRadius: 1,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
      color: fp.text.primary,
      cursor: 'pointer',
      '&:hover': { backgroundColor: fp.bg.hover },
    },
  };
};
