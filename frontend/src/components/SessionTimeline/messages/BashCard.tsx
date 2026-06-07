import { useState } from 'react';
import { Box, Collapse, Typography, useTheme } from '@mui/material';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import ExpandLessRoundedIcon from '@mui/icons-material/ExpandLessRounded';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import { useT } from '@/i18n';
import type { TimelineToolDetail } from '@/services';
import { sessionTimelineStyles } from '../SessionTimeline.styles';
import { CopyButton, formatTime } from './CopyButton';
import { createContentPreview } from './contentPreview';

// BashCard：把 Bash 结果的 stdout / stderr 分离渲染 —— stdout 走普通 pre，stderr 走 danger 调块。
// 有 stderr 时默认展开（错误要让人看见），图标也转 danger 色。数据来自 ToolResultDetail（kind==='bash'）。
export function BashCard({
  detail,
  label,
  timestamp,
}: {
  detail: TimelineToolDetail;
  label: string;
  timestamp?: string;
}) {
  const theme = useTheme();
  const s = sessionTimelineStyles(theme);
  const t = useT();
  const fp = theme.palette.foundation;
  const stdout = detail.stdout ?? '';
  const stderr = detail.stderr ?? '';
  const hasErr = stderr.trim().length > 0;
  const [open, setOpen] = useState(hasErr);
  const time = formatTime(timestamp);
  const preview = createContentPreview(hasErr ? stderr : stdout, 5);
  const showPreview = !open && preview.hasMore;
  return (
    <Box sx={s.block}>
      <Box sx={s.blockHeader} onClick={() => setOpen((o) => !o)}>
        <Box
          component="span"
          sx={{ display: 'inline-flex', flexShrink: 0, color: hasErr ? fp.status.danger : fp.accent }}
        >
          <TerminalRoundedIcon fontSize="small" />
        </Box>
        <Typography component="span" sx={s.blockHeaderLabel}>
          {label}
        </Typography>
        {time ? (
          <Box component="span" sx={s.timestamp}>
            {time}
          </Box>
        ) : null}
        <CopyButton text={stderr ? `${stdout}\n${stderr}` : stdout} sx={s.copyBtnInline} />
        {open ? <ExpandLessRoundedIcon fontSize="small" /> : <ExpandMoreRoundedIcon fontSize="small" />}
      </Box>
      {showPreview ? (
        <Box sx={s.previewWrap}>
          <Box component="pre" sx={s.previewText}>
            {preview.preview}
          </Box>
          <Box sx={s.moreIndicator}>
            {t('sessionTimeline.moreLines', { n: String(preview.totalLines - preview.previewLines) })}
          </Box>
        </Box>
      ) : null}
      <Collapse in={open} unmountOnExit>
        <Box component="pre" sx={s.pre}>
          {stdout || t('sessionTimeline.bash.empty')}
        </Box>
        {hasErr ? (
          <Box component="pre" sx={s.bashStderr}>
            <Box component="span" sx={s.bashStderrLabel}>
              {t('sessionTimeline.bash.stderr')}
            </Box>
            {stderr}
          </Box>
        ) : null}
      </Collapse>
    </Box>
  );
}
