import { useState } from 'react';
import { Box, Collapse, Typography, useTheme } from '@mui/material';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import ExpandLessRoundedIcon from '@mui/icons-material/ExpandLessRounded';
import DriveFileRenameOutlineRoundedIcon from '@mui/icons-material/DriveFileRenameOutlineRounded';
import { useT } from '@/i18n';
import type { TimelineToolDetail } from '@/services';
import { sessionTimelineStyles } from '../SessionTimeline.styles';
import { CopyButton } from './CopyButton';
import { formatTime } from './time';
import { mergeSx } from './sx';
import { basename } from './toolMeta';

// DiffCard：把 Edit/Write 的 structuredPatch 渲染成带行级 +/- 着色的差异卡（header 显示文件名 + +N/-N）。
// 短 diff（≤20 行）默认展开，长 diff 折叠。数据来自后端 transcript.ToolResultDetail（kind==='edit'）。
export function DiffCard({ detail, timestamp }: { detail: TimelineToolDetail; timestamp?: string }) {
  const theme = useTheme();
  const s = sessionTimelineStyles(theme);
  const t = useT();
  const lines = detail.diff ?? [];
  const [open, setOpen] = useState(lines.length <= 20);
  const time = formatTime(timestamp);
  const name = basename(detail.filePath) || t('sessionTimeline.result');
  return (
    <Box sx={s.block}>
      <Box sx={s.blockHeader} onClick={() => setOpen((o) => !o)}>
        <Box
          component="span"
          sx={{ display: 'inline-flex', flexShrink: 0, color: theme.palette.foundation.accent }}
        >
          <DriveFileRenameOutlineRoundedIcon fontSize="small" />
        </Box>
        <Typography component="span" sx={s.blockHeaderLabel}>
          {name}
        </Typography>
        <Box sx={s.diffSummaryRow}>
          {detail.added ? (
            <Box component="span" sx={s.diffStatAdd}>
              +{detail.added}
            </Box>
          ) : null}
          {detail.removed ? (
            <Box component="span" sx={s.diffStatDel}>
              -{detail.removed}
            </Box>
          ) : null}
        </Box>
        {time ? (
          <Box component="span" sx={s.timestamp}>
            {time}
          </Box>
        ) : null}
        <CopyButton text={lines.join('\n')} sx={s.copyBtnInline} />
        {open ? <ExpandLessRoundedIcon fontSize="small" /> : <ExpandMoreRoundedIcon fontSize="small" />}
      </Box>
      <Collapse in={open} unmountOnExit>
        <Box component="pre" sx={s.diffBody}>
          {lines.map((line, i) => {
            const lineSx = line.startsWith('+')
              ? s.diffLineAdd
              : line.startsWith('-')
                ? s.diffLineDel
                : undefined;
            return (
              <Box component="span" key={i} sx={mergeSx(s.diffLine, lineSx)}>
                {line || ' '}
              </Box>
            );
          })}
        </Box>
        {detail.truncated ? <Box sx={s.diffTruncated}>{t('sessionTimeline.diff.truncated')}</Box> : null}
      </Collapse>
    </Box>
  );
}
