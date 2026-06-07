import { Box, Typography, useTheme } from '@mui/material';
import FactCheckRoundedIcon from '@mui/icons-material/FactCheckRounded';
import { useT } from '@/i18n';
import { sessionTimelineStyles } from '../SessionTimeline.styles';
import { CopyButton, formatTime } from './CopyButton';

// PlanCard：把 ExitPlanMode 的 input.plan 渲染成醒目的「实施计划」卡（accent 边框 + 计划正文）。
// plan 缺失返回 null（调用方回退普通工具卡）。正文是 prose（非等宽），保留换行。
export function PlanCard({
  input,
  timestamp,
}: {
  input: Record<string, unknown> | null;
  timestamp?: string;
}) {
  const theme = useTheme();
  const s = sessionTimelineStyles(theme);
  const t = useT();
  const plan = typeof input?.plan === 'string' ? (input?.plan as string) : '';
  if (!plan.trim()) return null;
  const time = formatTime(timestamp);
  return (
    <Box sx={s.planCard}>
      <Box sx={s.planHeader}>
        <FactCheckRoundedIcon fontSize="small" />
        <Typography component="span" sx={s.planHeaderText}>
          {t('sessionTimeline.plan.title')}
        </Typography>
        {time ? (
          <Box component="span" sx={s.timestamp}>
            {time}
          </Box>
        ) : null}
        <CopyButton text={plan} sx={s.copyBtnInline} />
      </Box>
      <Box component="pre" sx={s.planBody}>
        {plan}
      </Box>
    </Box>
  );
}
