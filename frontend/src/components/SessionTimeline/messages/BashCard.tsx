import { useState } from 'react';
import { Box, Collapse, Typography, useTheme } from '@mui/material';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import ExpandLessRoundedIcon from '@mui/icons-material/ExpandLessRounded';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import { useT } from '@/i18n';
import type { TimelineToolDetail } from '@/services';
import { sessionTimelineStyles } from '../SessionTimeline.styles';
import { CopyButton } from './CopyButton';
import { formatTime } from './time';
import { createContentPreview } from './contentPreview';

// BashCard：把 Bash 结果的 stdout / stderr 分离渲染 —— stdout 走普通 pre，stderr 走 danger 调块。
// 「出错」以权威标志 isError（后端 <tool_use_error> 置位）判定，而非「stderr 非空」—— 大量命令把进度/告警
// 写 stderr 仍成功，避免它们被染红 + 强制展开。真出错才默认展开并把图标转 danger。stderr 有内容即渲染（供查看）。
export function BashCard({
  detail,
  label,
  isError,
  timestamp,
}: {
  detail: TimelineToolDetail;
  label: string;
  isError?: boolean;
  timestamp?: string;
}) {
  const theme = useTheme();
  const s = sessionTimelineStyles(theme);
  const t = useT();
  const fp = theme.palette.foundation;
  const stdout = detail.stdout ?? '';
  const stderr = detail.stderr ?? '';
  const showStderr = stderr.trim().length > 0;
  const isErr = isError === true;
  const [open, setOpen] = useState(isErr); // 真出错默认展开；含 stderr 但成功的命令不打扰
  const time = formatTime(timestamp);
  const preview = createContentPreview(isErr && showStderr ? stderr : stdout, 5);
  const showPreview = !open && preview.hasMore;
  return (
    <Box sx={s.block}>
      <Box sx={s.blockHeader} onClick={() => setOpen((o) => !o)}>
        <Box
          component="span"
          sx={{ display: 'inline-flex', flexShrink: 0, color: isErr ? fp.status.danger : fp.accent }}
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
        <CopyButton text={showStderr ? `${stdout}\n${stderr}` : stdout} sx={s.copyBtnInline} />
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
        {showStderr ? (
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
