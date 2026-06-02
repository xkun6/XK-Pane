import { useEffect, useRef, useState } from 'react';
import { Box, Collapse, IconButton, TextField, Tooltip, Typography, useTheme } from '@mui/material';
import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import PsychologyRoundedIcon from '@mui/icons-material/PsychologyRounded';
import BuildRoundedIcon from '@mui/icons-material/BuildRounded';
import SubdirectoryArrowRightRoundedIcon from '@mui/icons-material/SubdirectoryArrowRightRounded';
import SendRoundedIcon from '@mui/icons-material/SendRounded';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import ExpandLessRoundedIcon from '@mui/icons-material/ExpandLessRounded';
import { useT } from '@/i18n';
import type { TimelineEvent } from '@/services';
import { sessionTimelineStyles } from './SessionTimeline.styles';
import { useSessionTimeline } from './useSessionTimeline';

export interface SessionTimelineProps {
  sessionId: string;
  visible: boolean; // false=终端态：不渲染覆盖层、不轮询（保持挂载、状态不丢）
  onSend: (text: string) => void; // 把整句写进 PTY（父组件拼 '\r' 提交给 Agent）
}

// 工具输入摘要（折叠态一行预览）：挑最有信息量的字段。
function summarize(ev: TimelineEvent): string {
  if (!ev.toolInput) return '';
  try {
    const o = JSON.parse(ev.toolInput) as Record<string, unknown>;
    const pick = (k: string): string => (typeof o[k] === 'string' ? (o[k] as string) : '');
    return (
      pick('command') ||
      pick('file_path') ||
      pick('path') ||
      pick('pattern') ||
      pick('url') ||
      pick('description') ||
      pick('prompt')
    );
  } catch {
    return '';
  }
}

function prettyInput(ev: TimelineEvent): string {
  if (!ev.toolInput) return '';
  try {
    return JSON.stringify(JSON.parse(ev.toolInput), null, 2);
  } catch {
    return ev.toolInput;
  }
}

// 可折叠卡片（思考 / 工具调用 / 工具结果共用）。自带展开态，是纯展示子组件。
function CollapsibleBlock(props: {
  icon: React.ReactNode;
  label: string;
  summary?: string;
  bodyText: string;
  italic?: boolean;
  defaultOpen?: boolean;
}) {
  const theme = useTheme();
  const styles = sessionTimelineStyles(theme);
  const [open, setOpen] = useState(props.defaultOpen ?? false);
  return (
    <Box sx={styles.block}>
      <Box sx={styles.blockHeader} onClick={() => setOpen((o) => !o)}>
        {props.icon}
        <Typography component="span" sx={styles.blockHeaderLabel}>
          {props.label}
        </Typography>
        {!open && props.summary ? (
          <Typography component="span" sx={styles.blockSummary}>
            {props.summary}
          </Typography>
        ) : null}
        {open ? <ExpandLessRoundedIcon fontSize="small" /> : <ExpandMoreRoundedIcon fontSize="small" />}
      </Box>
      <Collapse in={open} unmountOnExit>
        <Box component="pre" sx={props.italic ? styles.thinkingText : styles.pre}>
          {props.bodyText}
        </Box>
      </Collapse>
    </Box>
  );
}

// SessionTimeline：把活跃会话的 Claude 转录渲染成可重排的对话卡片（覆盖在 xterm 之上）。
// supported!==true（加载中 / 非 claude）时不渲染 → 露出底层 xterm 镜像（零干扰）。
export const SessionTimeline = ({ sessionId, visible, onSend }: SessionTimelineProps) => {
  const theme = useTheme();
  const styles = sessionTimelineStyles(theme);
  const t = useT();
  const { events, title, supported } = useSessionTimeline({ sessionId, active: visible });
  const scrollRef = useRef<HTMLDivElement>(null);
  const [text, setText] = useState('');

  // 新事件到达 / 重新可见 → 贴底（监控场景默认看最新）。
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [events.length, visible]);

  // 终端态（!visible）或无结构化转录（非 claude）→ 不渲染覆盖层、露出底层 xterm；但保持挂载，
  // hook 已累积的事件 / supported 不丢，切回卡片即时复现（无需从头重拉）。
  if (supported !== true || !visible) return null;

  const send = (): void => {
    const v = text.trim();
    if (!v) return;
    onSend(v);
    setText('');
  };
  const onKeyDown = (e: React.KeyboardEvent): void => {
    // 回车发送，Shift+Enter 换行；输入法合成中（中文/日文）不抢回车。
    if (e.key === 'Enter' && !e.shiftKey && !(e.nativeEvent as { isComposing?: boolean }).isComposing) {
      e.preventDefault();
      send();
    }
  };

  const renderEvent = (ev: TimelineEvent, idx: number): React.ReactNode => {
    const key = `${ev.seq}-${ev.kind}-${idx}`;
    switch (ev.kind) {
      case 'user':
        return (
          <Box key={key} sx={styles.userRow}>
            <Box sx={styles.userBubble}>{ev.text}</Box>
          </Box>
        );
      case 'assistant':
        return (
          <Box key={key} sx={styles.assistantBubble}>
            {ev.text}
          </Box>
        );
      case 'thinking':
        return (
          <CollapsibleBlock
            key={key}
            icon={<PsychologyRoundedIcon fontSize="small" />}
            label={t('sessionTimeline.thinking')}
            bodyText={ev.text ?? ''}
            italic
          />
        );
      case 'tool_use':
        return (
          <CollapsibleBlock
            key={key}
            icon={<BuildRoundedIcon fontSize="small" />}
            label={ev.toolName || t('sessionTimeline.tool')}
            summary={summarize(ev)}
            bodyText={prettyInput(ev)}
          />
        );
      case 'tool_result':
        return (
          <CollapsibleBlock
            key={key}
            icon={<SubdirectoryArrowRightRoundedIcon fontSize="small" />}
            label={t('sessionTimeline.result')}
            bodyText={ev.text ?? ''}
          />
        );
      default:
        return null;
    }
  };

  return (
    <Box sx={styles.overlayRoot}>
      <Box sx={styles.header}>
        <AutoAwesomeRoundedIcon fontSize="small" />
        <Typography component="span" sx={styles.headerTitle}>
          {title || t('sessionTimeline.title')}
        </Typography>
      </Box>

      {events.length === 0 ? (
        <Box sx={styles.empty}>{t('sessionTimeline.empty')}</Box>
      ) : (
        <Box ref={scrollRef} sx={styles.scroll}>
          {events.map(renderEvent)}
        </Box>
      )}

      <Box sx={styles.composeBar}>
        <TextField
          sx={styles.composeField}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder={t('sessionTimeline.composePlaceholder')}
          multiline
          maxRows={4}
          size="small"
          slotProps={{ htmlInput: { 'aria-label': t('sessionTimeline.composePlaceholder'), enterKeyHint: 'send' } }}
        />
        <Tooltip title={t('sessionTimeline.send')}>
          <span>
            <IconButton color="primary" onClick={send} disabled={!text.trim()} aria-label={t('sessionTimeline.send')}>
              <SendRoundedIcon />
            </IconButton>
          </span>
        </Tooltip>
      </Box>
    </Box>
  );
};
