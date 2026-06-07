import { useState } from 'react';
import { Box, Collapse, Typography, useTheme } from '@mui/material';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import ExpandLessRoundedIcon from '@mui/icons-material/ExpandLessRounded';
import { useT } from '@/i18n';
import { sessionTimelineStyles, resolveTone, type BlockTone } from '../SessionTimeline.styles';
import { CopyButton } from './CopyButton';
import { formatTime } from './time';
import { createContentPreview } from './contentPreview';

export interface CollapsibleBlockProps {
  icon: React.ReactNode;
  label: string;
  summary?: string; // 折叠态 header 一行预览（如工具输入的 command/path）
  bodyText: string;
  italic?: boolean; // 思考正文用斜体灰
  defaultOpen?: boolean;
  tone?: BlockTone; // 图标色调
  timestamp?: string;
  previewLines?: number; // 折叠态展示正文前 N 行（0/省略=不展示，适合 JSON 等已有 summary 的）
}

// CollapsibleBlock：可折叠卡片（思考 / 工具调用 / 工具结果共用）。折叠态可选展示首 N 行预览
// （对标 claude-code-webui，让折叠卡不只剩一条标题横杠）。纯展示子组件。
export function CollapsibleBlock(props: CollapsibleBlockProps) {
  const theme = useTheme();
  const styles = sessionTimelineStyles(theme);
  const t = useT();
  const [open, setOpen] = useState(props.defaultOpen ?? false);
  const toneColor = resolveTone(theme, props.tone);
  const time = formatTime(props.timestamp);
  const preview =
    props.previewLines && props.previewLines > 0
      ? createContentPreview(props.bodyText, props.previewLines)
      : null;
  const showPreview = !open && !!preview && preview.hasMore;
  return (
    <Box sx={styles.block}>
      <Box sx={styles.blockHeader} onClick={() => setOpen((o) => !o)}>
        <Box component="span" sx={{ display: 'inline-flex', flexShrink: 0, color: toneColor }}>
          {props.icon}
        </Box>
        <Typography component="span" sx={styles.blockHeaderLabel}>
          {props.label}
        </Typography>
        {!open && props.summary ? (
          <Typography component="span" sx={styles.blockSummary}>
            {props.summary}
          </Typography>
        ) : null}
        {time ? (
          <Box component="span" sx={styles.timestamp}>
            {time}
          </Box>
        ) : null}
        <CopyButton text={props.bodyText} sx={styles.copyBtnInline} />
        {open ? <ExpandLessRoundedIcon fontSize="small" /> : <ExpandMoreRoundedIcon fontSize="small" />}
      </Box>
      {showPreview && preview ? (
        <Box sx={styles.previewWrap}>
          <Box component="pre" sx={styles.previewText}>
            {preview.preview}
          </Box>
          <Box sx={styles.moreIndicator}>
            {t('sessionTimeline.moreLines', { n: String(preview.totalLines - preview.previewLines) })}
          </Box>
        </Box>
      ) : null}
      <Collapse in={open} unmountOnExit>
        <Box component="pre" sx={props.italic ? styles.thinkingText : styles.pre}>
          {props.bodyText}
        </Box>
      </Collapse>
    </Box>
  );
}
