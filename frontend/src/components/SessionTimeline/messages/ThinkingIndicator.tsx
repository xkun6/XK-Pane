import { Box, CircularProgress, Typography, useTheme } from '@mui/material';
import { useT } from '@/i18n';
import { sessionTimelineStyles } from '../SessionTimeline.styles';

// ThinkingIndicator：会话「工作中」时置于时间线末尾的 liveness 指示器（自旋 + 「思考中…」），
// 让用户在卡片视图里也能确信 Agent 仍在跑、未因意外中断（对标 claude-code-webui 的 LoadingComponent）。
export function ThinkingIndicator() {
  const theme = useTheme();
  const s = sessionTimelineStyles(theme);
  const t = useT();
  return (
    <Box sx={s.workingBubble}>
      <CircularProgress size={16} thickness={5} sx={{ color: 'currentColor' }} aria-hidden />
      <Typography component="span" sx={s.workingText}>
        {t('sessionTimeline.working')}
      </Typography>
    </Box>
  );
}
