import { Box, Typography, useTheme } from '@mui/material';
import type { SxProps, Theme } from '@mui/material';
import { useT } from '@/i18n';
import type { TimelineMetrics } from '@/services';
import { sessionTimelineStyles } from './SessionTimeline.styles';

// StatusLine：卡片底部输入框下方的状态行，复刻 claude-hud 的「上下文 / 用量(5h) / 本周」三项
// 纯文字指标（无进度块）。数字与取色阈值与 claude-hud 完全一致（数据即读自其落盘快照，见后端 hudstats）。
// 作为覆盖层最底元素：始终渲染最外层容器以承载移动端安全区垫高（原 composeBar 的安全区下移到此），
// 无指标时内层不渲染 → 桌面下零高度不可见。

// formatTokens：对齐 claude-hud identity.ts formatTokens（≥1e6→"1.0M"，≥1e3→"216k"）。
function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}k`;
  return String(n);
}

// formatRemaining：剩余时间 = resetsAtMs − now，对齐 claude-hud format-reset-time.ts formatRelative。
// 过期 / 未知返回 ''（不显示括号时间）。
function formatRemaining(resetsAtMs?: number): string {
  if (!resetsAtMs) return '';
  const diffMs = resetsAtMs - Date.now();
  if (diffMs <= 0) return '';
  const diffMins = Math.ceil(diffMs / 60000);
  if (diffMins < 60) return `${diffMins}m`;
  const hours = Math.floor(diffMins / 60);
  const mins = diffMins % 60;
  if (hours >= 24) {
    const days = Math.floor(hours / 24);
    const remHours = hours % 24;
    return remHours > 0 ? `${days}d ${remHours}h` : `${days}d`;
  }
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

// 上下文取色：≥85 危险 / ≥70 警告 / 否则正常（绿），对齐 claude-hud getContextColor。
function contextColor(theme: Theme, pct: number): string {
  const fp = theme.palette.foundation;
  if (pct >= 85) return fp.status.danger;
  if (pct >= 70) return fp.status.warning;
  return fp.status.success;
}

// 用量取色：≥90 危险 / ≥75 警告 / 否则正常（accent），对齐 claude-hud getQuotaColor。
function quotaColor(theme: Theme, pct: number): string {
  const fp = theme.palette.foundation;
  if (pct >= 90) return fp.status.danger;
  if (pct >= 75) return fp.status.warning;
  return fp.accent;
}

interface StatusItemProps {
  label: string;
  value: string;
  color: string;
  styles: Record<string, SxProps<Theme>>;
}

const StatusItem = ({ label, value, color, styles }: StatusItemProps) => (
  <Box sx={styles.statusItem}>
    <Typography component="span" sx={styles.statusLabel}>
      {label}
    </Typography>
    <Typography component="span" sx={[styles.statusValue, { color }] as SxProps<Theme>}>
      {value}
    </Typography>
  </Box>
);

export interface StatusLineProps {
  metrics: TimelineMetrics | null;
}

export const StatusLine = ({ metrics }: StatusLineProps) => {
  const theme = useTheme();
  const styles = sessionTimelineStyles(theme);
  const t = useT();

  const items: React.ReactNode[] = [];

  const ctx = metrics?.context;
  if (ctx && ctx.windowSize > 0) {
    items.push(
      <StatusItem
        key="ctx"
        label={t('sessionTimeline.status.context')}
        value={`${formatTokens(ctx.usedTokens)}/${formatTokens(ctx.windowSize)}`}
        color={contextColor(theme, ctx.usedPercentage)}
        styles={styles}
      />,
    );
  }

  const five = metrics?.fiveHour;
  if (five) {
    const rem = formatRemaining(five.resetsAtMs);
    items.push(
      <StatusItem
        key="5h"
        label={t('sessionTimeline.status.usage5h')}
        value={rem ? `${five.usedPercentage}% (${rem})` : `${five.usedPercentage}%`}
        color={quotaColor(theme, five.usedPercentage)}
        styles={styles}
      />,
    );
  }

  const seven = metrics?.sevenDay;
  if (seven) {
    const rem = formatRemaining(seven.resetsAtMs);
    items.push(
      <StatusItem
        key="wk"
        label={t('sessionTimeline.status.weekly')}
        value={rem ? `${seven.usedPercentage}% (${rem})` : `${seven.usedPercentage}%`}
        color={quotaColor(theme, seven.usedPercentage)}
        styles={styles}
      />,
    );
  }

  return <Box sx={styles.statusBar}>{items.length > 0 && <Box sx={styles.statusRow}>{items}</Box>}</Box>;
};
