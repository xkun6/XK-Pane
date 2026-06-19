import type { SxProps, Theme } from '@mui/material';

// 侧栏底部「token 用量」竖向三格（设置齿轮上方）：
//  - 每格 = 一个时间窗口（近5h / 今日 / 本周），上行 token 总量（粗体、tabular-nums）、下行小标签
//  - 整组顶部一条细 divider 与上方导航区分隔；hover 弹完整明细 tooltip
//  - 配色全取 theme.palette.foundation.*，三主题自适配；圆角 8（容器语言）
export const agentUsageRailStyles = (theme: Theme): Record<string, SxProps<Theme>> => {
  const fp = theme.palette.foundation;
  return {
    root: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 0.5, // 4px
      width: 48,
      mb: 0.5,
    },
    divider: {
      width: 28,
      height: '1px',
      backgroundColor: fp.divider,
      mb: 0.5,
    },
    cell: {
      width: 46,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: '1px',
      py: 0.5, // 4px
      borderRadius: 1, // 8px
      backgroundColor: fp.bg.elevated,
      cursor: 'default',
    },
    value: {
      fontSize: 12.5,
      fontWeight: 700,
      lineHeight: 1.1,
      color: fp.text.primary,
      fontVariantNumeric: 'tabular-nums',
    },
    label: {
      fontSize: 9,
      lineHeight: 1.1,
      color: fp.text.secondary,
      letterSpacing: 0.2,
    },
    // —— Tooltip 明细面板（覆盖 MUI 默认深色气泡，改用 foundation 表面）——
    tooltipSurface: {
      backgroundColor: fp.bg.elevated,
      color: fp.text.primary,
      border: `1px solid ${fp.divider}`,
      borderRadius: 2, // 8px
      boxShadow: theme.shadows[6],
      p: 1.25,
      maxWidth: 260,
    },
    tipTitle: {
      fontSize: 12,
      fontWeight: 700,
      color: fp.text.primary,
      mb: 0.75,
    },
    tipRow: {
      display: 'flex',
      flexDirection: 'column',
      gap: '1px',
      mb: 0.75,
    },
    tipRowHead: {
      display: 'flex',
      alignItems: 'baseline',
      justifyContent: 'space-between',
      gap: 1.5,
    },
    tipRowLabel: {
      fontSize: 11.5,
      color: fp.text.secondary,
    },
    tipRowValue: {
      fontSize: 12.5,
      fontWeight: 700,
      color: fp.text.primary,
      fontVariantNumeric: 'tabular-nums',
    },
    tipBreakdown: {
      fontSize: 10.5,
      color: fp.text.secondary,
      opacity: 0.85,
    },
    tipFoot: {
      fontSize: 10,
      color: fp.text.secondary,
      opacity: 0.7,
      mt: 0.25,
    },
  };
};
