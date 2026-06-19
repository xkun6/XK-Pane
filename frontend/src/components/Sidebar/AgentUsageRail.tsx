import { Box, Tooltip, Typography, useTheme } from '@mui/material';
import { useT } from '@/i18n';
import type { TokenWindow } from '@/services';
import { agentUsageRailStyles } from './AgentUsageRail.styles';
import { useAgentUsage } from './useAgentUsage';

// token 数缩写（对齐 fanbox fmtTok）：>=1e9→B / >=1e6→M / >=1e3→k，小数仅在 1 位档保留。
const fmtTok = (n: number): string => {
  if (n >= 1e9) return `${(n / 1e9).toFixed(n < 1e10 ? 1 : 0)}B`;
  if (n >= 1e6) return `${(n / 1e6).toFixed(n < 1e7 ? 1 : 0)}M`;
  if (n >= 1e3) return `${(n / 1e3).toFixed(0)}k`;
  return String(Math.round(n));
};

// 侧栏底部「Claude Code token 用量」竖向四格：本机会话日志统计出的近5h/今日/本周/本月 token 总量，
// 叠在设置齿轮上方（自上而下 近5h→今日→本周→本月，本月最近齿轮）。hover 弹完整明细（含输入/输出/缓存拆分）。
// 仅桌面端有数据（viewer 无本机文件系统 → useAgentUsage 不轮询，usage 恒 null，整组不渲染）。
export const AgentUsageRail = () => {
  const theme = useTheme();
  const styles = agentUsageRailStyles(theme);
  const t = useT();
  const { usage } = useAgentUsage();

  // 没装/没用过 Claude Code，或近一月/一周都无任何用量 → 不显示（避免一排 0 的噪声）。
  // 本月与本周是不同口径（自然月 vs 近 7 天滑动），月初两者可分叉，故取二者皆为 0 才隐藏。
  if (!usage || !usage.available || (usage.week.total <= 0 && usage.month.total <= 0)) {
    return null;
  }

  // 自上而下：近5h / 今日 / 本周 / 本月 —— 本月紧贴下方的设置齿轮（「5h 在最上、本月在最下」）。
  const cells: { key: string; labelKey: string; w: TokenWindow }[] = [
    { key: 'last5h', labelKey: 'sidebar.usage.last5h', w: usage.last5h },
    { key: 'today', labelKey: 'sidebar.usage.today', w: usage.today },
    { key: 'week', labelKey: 'sidebar.usage.week', w: usage.week },
    { key: 'month', labelKey: 'sidebar.usage.month', w: usage.month },
  ];

  // Tooltip 明细：近5h → 今日 → 本周 → 本月（递增窗口的自然阅读序），每窗口一行总量 + 一行四类 token 拆分。
  const tipWindows: { labelKey: string; w: TokenWindow }[] = [
    { labelKey: 'sidebar.usage.last5h', w: usage.last5h },
    { labelKey: 'sidebar.usage.today', w: usage.today },
    { labelKey: 'sidebar.usage.week', w: usage.week },
    { labelKey: 'sidebar.usage.month', w: usage.month },
  ];

  const tooltip = (
    <Box>
      <Typography component="div" sx={styles.tipTitle}>
        {t('sidebar.usage.title')}
      </Typography>
      {tipWindows.map(({ labelKey, w }) => (
        <Box key={labelKey} sx={styles.tipRow}>
          <Box sx={styles.tipRowHead}>
            <Typography component="span" sx={styles.tipRowLabel}>
              {t(labelKey)}
            </Typography>
            <Typography component="span" sx={styles.tipRowValue}>
              {fmtTok(w.total)}
            </Typography>
          </Box>
          <Typography component="div" sx={styles.tipBreakdown}>
            {`${t('sidebar.usage.input')} ${fmtTok(w.input)} · ${t('sidebar.usage.output')} ${fmtTok(
              w.output,
            )} · ${t('sidebar.usage.cacheRead')} ${fmtTok(w.cacheRead)} · ${t(
              'sidebar.usage.cacheCreate',
            )} ${fmtTok(w.cacheCreate)} · ${fmtTok(w.msgs)} ${t('sidebar.usage.msgs')}`}
          </Typography>
        </Box>
      ))}
      <Typography component="div" sx={styles.tipFoot}>
        {t('sidebar.usage.source')}
      </Typography>
    </Box>
  );

  return (
    <Tooltip
      title={tooltip}
      placement="right"
      slotProps={{ tooltip: { sx: styles.tooltipSurface } }}
    >
      <Box sx={styles.root} aria-label={t('sidebar.usage.ariaLabel')}>
        <Box sx={styles.divider} aria-hidden />
        {cells.map(({ key, labelKey, w }) => (
          <Box key={key} sx={styles.cell}>
            <Typography component="span" sx={styles.value}>
              {fmtTok(w.total)}
            </Typography>
            <Typography component="span" sx={styles.label}>
              {t(labelKey)}
            </Typography>
          </Box>
        ))}
      </Box>
    </Tooltip>
  );
};
