import type { SxProps, Theme } from '@mui/material';

export const homePageStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  const isDark = theme.palette.mode === 'dark';
  const cardShadow = isDark
    ? '0 1px 2px rgba(0,0,0,0.28), 0 0 0 1px rgba(255,255,255,0.03)'
    : '0 1px 2px rgba(15, 23, 42, 0.04), 0 4px 12px rgba(15, 23, 42, 0.04)';
  const cardHoverShadow = isDark
    ? '0 4px 16px rgba(0,0,0,0.4), 0 0 0 1px rgba(255,255,255,0.05)'
    : '0 4px 16px rgba(15, 23, 42, 0.08), 0 1px 3px rgba(15, 23, 42, 0.04)';

  return {
    root: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      backgroundColor: fp.bg.content,
      // 滚动条归外层撑满宽度的 root；内容限宽交给 body
      overflowY: 'auto',
    },
    body: {
      px: { xs: 3, sm: 5, md: 6 },
      py: { xs: 3.5, sm: 5 },
      display: 'flex',
      flexDirection: 'column',
      gap: 4.5,
      maxWidth: 960,
      width: '100%',
      mx: 'auto',
      boxSizing: 'border-box',
    },
    // —— 问候头 ——
    greetingRow: {
      display: 'flex',
      alignItems: 'center',
      flexWrap: 'wrap',
      gap: 1.5,
    },
    greeting: {
      fontSize: { xs: 26, sm: 32 },
      fontWeight: 700,
      color: fp.text.primary,
      lineHeight: 1.15,
      letterSpacing: '-0.025em',
      m: 0,
    },
    statChip: {
      fontSize: 12.5,
      fontWeight: 500,
      color: fp.text.secondary,
      px: 1.25,
      py: 0.4,
      borderRadius: '999px',
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
      boxShadow: cardShadow,
    },
    tagline: {
      color: fp.text.secondary,
      fontSize: 15,
      lineHeight: 1.6,
      maxWidth: 520,
      mt: 0.25,
    },
    // —— 分区 ——
    section: {
      display: 'flex',
      flexDirection: 'column',
      gap: 1.5,
    },
    sectionTitle: {
      fontSize: 11.5,
      fontWeight: 600,
      letterSpacing: 1.1,
      textTransform: 'uppercase',
      color: fp.text.muted,
      px: 0.25,
    },
    // —— 快捷操作 ——
    actionsGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(168px, 1fr))',
      gap: 1.5,
    },
    actionCard: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      p: 2,
      borderRadius: '14px',
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
      boxShadow: cardShadow,
      cursor: 'pointer',
      transition:
        'background-color 160ms cubic-bezier(0.4, 0, 0.2, 1), border-color 160ms ease, box-shadow 160ms ease, transform 120ms ease',
      '&:hover': {
        backgroundColor: fp.bg.surface,
        borderColor: isDark ? 'rgba(96, 165, 250, 0.35)' : 'rgba(59, 130, 246, 0.28)',
        boxShadow: cardHoverShadow,
        transform: 'translateY(-1px)',
      },
      '&:active': {
        transform: 'translateY(0)',
      },
    },
    actionIcon: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: 40,
      height: 40,
      borderRadius: '11px',
      color: fp.accent,
      backgroundColor: fp.bg.active,
      flexShrink: 0,
      '& .MuiSvgIcon-root': { fontSize: 22 },
    },
    actionLabel: {
      fontSize: 14,
      fontWeight: 600,
      color: fp.text.primary,
      letterSpacing: '-0.01em',
    },
    // —— 最近启动 ——
    recentList: {
      display: 'flex',
      flexDirection: 'column',
      gap: 0.75,
    },
    recentRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      padding: '12px 14px',
      borderRadius: '12px',
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
      boxShadow: cardShadow,
      cursor: 'pointer',
      transition:
        'background-color 140ms ease, border-color 140ms ease, box-shadow 140ms ease',
      '&:hover': {
        backgroundColor: fp.bg.hover,
        borderColor: isDark ? 'rgba(148,163,184,0.2)' : 'rgba(15,23,42,0.12)',
        boxShadow: cardHoverShadow,
      },
    },
    recentIcon: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: 32,
      height: 32,
      borderRadius: '9px',
      color: fp.text.secondary,
      backgroundColor: fp.bg.elevated,
      flexShrink: 0,
      '& .MuiSvgIcon-root': { fontSize: 18 },
    },
    recentName: {
      flex: 1,
      fontSize: 13.5,
      fontWeight: 500,
      color: fp.text.primary,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    recentTime: {
      fontSize: 12,
      color: fp.text.muted,
      whiteSpace: 'nowrap',
      fontVariantNumeric: 'tabular-nums',
    },
    // —— 环境检测 ——
    envList: {
      display: 'flex',
      flexDirection: 'column',
      gap: 0.75,
      p: 0.5,
      borderRadius: '14px',
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
      boxShadow: cardShadow,
    },
    envRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      padding: '10px 12px',
      borderRadius: '10px',
      backgroundColor: 'transparent',
      transition: 'background-color 120ms ease',
      '&:hover': {
        backgroundColor: fp.bg.hover,
      },
    },
    envDot: {
      width: 8,
      height: 8,
      borderRadius: '50%',
      flexShrink: 0,
      boxShadow: '0 0 0 3px rgba(0,0,0,0.04)',
    },
    envName: {
      flex: 1,
      fontSize: 13.5,
      fontWeight: 500,
      color: fp.text.primary,
    },
    envVersion: {
      fontSize: 12,
      fontFamily: 'ui-monospace, "JetBrains Mono", monospace',
      color: fp.text.secondary,
      whiteSpace: 'nowrap',
      px: 1,
      py: 0.25,
      borderRadius: '6px',
      backgroundColor: fp.bg.elevated,
    },
    emptyHint: {
      padding: '20px 12px',
      fontSize: 13,
      color: fp.text.muted,
      textAlign: 'center',
      borderRadius: '12px',
      border: `1px dashed ${fp.divider}`,
      backgroundColor: fp.bg.surface,
    },
  };
};
