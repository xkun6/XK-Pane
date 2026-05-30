import type { SxProps, Theme } from '@mui/material';

export const homePageStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    root: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      backgroundColor: fp.bg.content,
      overflow: 'hidden',
    },
    body: {
      flex: 1,
      overflowY: 'auto',
      px: 6,
      py: 5,
      display: 'flex',
      flexDirection: 'column',
      gap: 4,
      maxWidth: 920,
      width: '100%',
    },
    // —— 问候头 ——
    greetingRow: {
      display: 'flex',
      alignItems: 'baseline',
      flexWrap: 'wrap',
      gap: 1.5,
    },
    greeting: {
      fontSize: 30,
      fontWeight: 700,
      color: fp.text.primary,
      lineHeight: 1.2,
      m: 0,
    },
    statChip: {
      fontSize: 13,
      color: fp.text.secondary,
      px: 1,
      py: 0.25,
      borderRadius: 6,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
    },
    tagline: {
      color: fp.text.secondary,
      fontSize: 15,
      lineHeight: 1.6,
    },
    // —— 分区 ——
    section: {
      display: 'flex',
      flexDirection: 'column',
      gap: 1.25,
    },
    sectionTitle: {
      fontSize: 12,
      fontWeight: 600,
      letterSpacing: 1,
      textTransform: 'uppercase',
      color: fp.text.secondary,
    },
    // —— 快捷操作 ——
    actionsGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
      gap: 1.5,
    },
    actionCard: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      p: 2,
      borderRadius: 8,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
      cursor: 'pointer',
      transition: 'background-color 120ms, border-color 120ms',
      '&:hover': { backgroundColor: fp.bg.hover, borderColor: fp.accent },
    },
    actionIcon: {
      display: 'flex',
      color: fp.accent,
    },
    actionLabel: {
      fontSize: 14,
      fontWeight: 600,
      color: fp.text.primary,
    },
    // —— 最近启动 ——
    recentList: {
      display: 'flex',
      flexDirection: 'column',
      gap: 0.5,
    },
    recentRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      padding: '10px 14px',
      borderRadius: 8,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
      cursor: 'pointer',
      '&:hover': { backgroundColor: fp.bg.hover },
    },
    recentIcon: {
      display: 'flex',
      color: fp.text.secondary,
      flexShrink: 0,
    },
    recentName: {
      flex: 1,
      fontSize: 14,
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
    },
    // —— 环境检测 ——
    envList: {
      display: 'flex',
      flexDirection: 'column',
      gap: 0.5,
    },
    envRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      padding: '10px 14px',
      borderRadius: 8,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
    },
    envDot: {
      width: 8,
      height: 8,
      borderRadius: '50%',
      flexShrink: 0,
    },
    envName: {
      flex: 1,
      fontSize: 14,
      color: fp.text.primary,
    },
    envVersion: {
      fontSize: 12,
      fontFamily: 'monospace',
      color: fp.text.secondary,
      whiteSpace: 'nowrap',
    },
    emptyHint: {
      padding: '16px 4px',
      fontSize: 13,
      color: fp.text.secondary,
    },
  };
};
