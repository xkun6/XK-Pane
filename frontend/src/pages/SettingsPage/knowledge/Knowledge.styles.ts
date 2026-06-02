import type { SxProps, Theme } from '@mui/material';

// 知识库子页面专属样式：状态卡片 + 计数瓦片 + 源选择 + 同步进度/日志 + 代理/源表单 + 归档列表。
// 复用 SettingsPage.styles 的 section/sectionTitle/sectionHint；本文件只放卡片级样式。
export const knowledgeStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    // 通用卡片
    card: {
      display: 'flex',
      flexDirection: 'column',
      gap: 1.5,
      p: 2.5,
      borderRadius: 1.5,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
    },
    cardTitle: {
      fontSize: 13,
      fontWeight: 700,
      color: fp.text.primary,
    },
    cardHint: {
      fontSize: 11,
      color: fp.text.muted,
      lineHeight: 1.55,
    },

    // —— 状态：计数瓦片 ——
    statRow: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))',
      gap: 1.5,
    },
    statTile: {
      display: 'flex',
      flexDirection: 'column',
      gap: 0.25,
      p: 1.5,
      borderRadius: 1.25,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
    },
    statValue: {
      fontSize: 22,
      fontWeight: 700,
      color: fp.text.primary,
      lineHeight: 1.1,
    },
    statLabel: {
      fontSize: 11,
      color: fp.text.muted,
      letterSpacing: 0.3,
    },

    // 信息行（路径 / 大小 / 上次同步）
    infoRow: {
      display: 'flex',
      alignItems: 'baseline',
      gap: 2,
      flexWrap: 'wrap',
    },
    infoLabel: {
      fontSize: 12,
      color: fp.text.muted,
      letterSpacing: 0.4,
      textTransform: 'uppercase',
      fontWeight: 600,
      flexShrink: 0,
      width: 100,
    },
    infoValue: {
      fontSize: 13,
      color: fp.text.primary,
      flex: 1,
      minWidth: 0,
      wordBreak: 'break-all',
    },
    badge: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 0.5,
      fontSize: 11,
      fontWeight: 600,
      px: 1,
      py: 0.25,
      borderRadius: 0.75,
    },
    badgePresent: {
      backgroundColor: fp.bg.active,
      color: fp.accent,
    },
    badgeEmbedded: {
      backgroundColor: fp.bg.elevated,
      color: fp.text.secondary,
    },

    // —— 来源溯源行 ——
    sourceProvenanceRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      fontSize: 12,
    },
    sourceProvenanceName: {
      fontWeight: 600,
      color: fp.text.primary,
      minWidth: 80,
    },
    sourceProvenanceMeta: {
      color: fp.text.muted,
      wordBreak: 'break-all',
    },

    // —— 同步：源勾选 ——
    sourcePicker: {
      display: 'flex',
      gap: 2,
      flexWrap: 'wrap',
    },

    // 动作行
    actions: {
      display: 'flex',
      gap: 1,
      flexWrap: 'wrap',
      alignItems: 'center',
      mt: 0.5,
    },
    actionPrimary: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 0.75,
      fontSize: 13,
      fontWeight: 600,
      px: 2,
      py: 1,
      borderRadius: 1,
      border: `1px solid ${fp.accent}`,
      backgroundColor: fp.accent,
      color: '#fff',
      cursor: 'pointer',
      '&:hover': { backgroundColor: fp.accentHover },
      '&:disabled': { opacity: 0.6, cursor: 'not-allowed' },
    },
    actionSecondary: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 0.75,
      fontSize: 13,
      px: 2,
      py: 1,
      borderRadius: 1,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
      color: fp.text.primary,
      cursor: 'pointer',
      '&:hover': { backgroundColor: fp.bg.hover },
      '&:disabled': { opacity: 0.6, cursor: 'not-allowed' },
    },
    actionDanger: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 0.75,
      fontSize: 13,
      px: 2,
      py: 1,
      borderRadius: 1,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
      color: fp.status.danger,
      cursor: 'pointer',
      '&:hover': { backgroundColor: fp.bg.hover },
      '&:disabled': { opacity: 0.6, cursor: 'not-allowed' },
    },

    // —— 进度 ——
    progressBlock: {
      display: 'flex',
      flexDirection: 'column',
      gap: 1,
    },
    progressHeadline: {
      display: 'flex',
      alignItems: 'center',
      gap: 1,
      fontSize: 13,
      fontWeight: 600,
      color: fp.text.primary,
    },
    progressCount: {
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 12,
      color: fp.text.muted,
    },
    logBox: {
      mt: 0.5,
      p: 1.5,
      borderRadius: 1.25,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.elevated,
      maxHeight: 200,
      overflowY: 'auto',
      display: 'flex',
      flexDirection: 'column',
      gap: 0.25,
    },
    logLine: {
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 12,
      lineHeight: 1.5,
      color: fp.text.secondary,
      wordBreak: 'break-all',
    },
    logWarn: { color: fp.status.warning },
    logError: { color: fp.status.danger },

    // 状态提示
    status: {
      fontSize: 12,
      color: fp.text.muted,
      lineHeight: 1.5,
    },
    statusSuccess: {
      fontSize: 12,
      color: fp.status.success,
      lineHeight: 1.5,
    },
    statusError: {
      fontSize: 12,
      color: fp.status.danger,
      lineHeight: 1.5,
    },

    // 同步报告摘要
    reportRow: {
      display: 'flex',
      flexWrap: 'wrap',
      gap: 1,
      mt: 0.5,
    },
    reportChip: {
      fontSize: 11,
      fontWeight: 600,
      px: 1,
      py: 0.25,
      borderRadius: 0.75,
      backgroundColor: fp.bg.elevated,
      color: fp.text.secondary,
    },

    // —— 表单（代理 / 源）——
    formGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
      gap: 1.5,
    },
    formFull: {
      gridColumn: '1 / -1',
    },

    // —— 归档列表 ——
    archiveList: {
      display: 'flex',
      flexDirection: 'column',
      gap: 1,
    },
    archiveRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      p: 1.5,
      borderRadius: 1.25,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
    },
    archiveMain: {
      flex: 1,
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      gap: 0.25,
    },
    archiveName: {
      fontSize: 13,
      fontWeight: 600,
      color: fp.text.primary,
      wordBreak: 'break-all',
    },
    archiveMeta: {
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 12,
      color: fp.text.muted,
    },
  };
};
