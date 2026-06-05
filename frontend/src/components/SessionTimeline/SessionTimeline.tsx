import { useEffect, useRef, useState } from 'react';
import { Box, Collapse, IconButton, TextField, Tooltip, Typography, useTheme } from '@mui/material';
import type { SxProps, Theme } from '@mui/material';
import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import PsychologyRoundedIcon from '@mui/icons-material/PsychologyRounded';
import BuildRoundedIcon from '@mui/icons-material/BuildRounded';
import SubdirectoryArrowRightRoundedIcon from '@mui/icons-material/SubdirectoryArrowRightRounded';
import ErrorOutlineRoundedIcon from '@mui/icons-material/ErrorOutlineRounded';
import SendRoundedIcon from '@mui/icons-material/SendRounded';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import ExpandLessRoundedIcon from '@mui/icons-material/ExpandLessRounded';
import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import CheckRoundedIcon from '@mui/icons-material/CheckRounded';
import { useT } from '@/i18n';
import { copyText } from '@/shared/clipboard';
import type { TimelineEvent } from '@/services';
import { sessionTimelineStyles } from './SessionTimeline.styles';
import { useSessionTimeline } from './useSessionTimeline';
import { AskQuestionCard } from './AskQuestionCard';

export interface SessionTimelineProps {
  sessionId: string;
  visible: boolean; // false=终端态：不渲染覆盖层、不轮询（保持挂载、状态不丢）
  onSend: (text: string) => void; // 把整句写进 PTY（父组件拼 '\r' 提交给 Agent）
  onRawInput?: (data: string) => void; // 原样写入 PTY（不追加 '\r'）：AskUserQuestion 作答按键序列
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

// 复制按钮：把传入文本写进剪贴板（走 shared/clipboard，兼容 Web 镜像非安全上下文），
// 成功后图标短暂变勾反馈。显隐由父卡片样式经 .xk-copy-btn 控制（hover 显示 / touch 常驻）。
function CopyButton({ text, sx }: { text: string; sx?: SxProps<Theme> }) {
  const t = useT();
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const id = setTimeout(() => setCopied(false), 1500);
    return () => clearTimeout(id);
  }, [copied]);
  if (!text) return null; // 空内容不给复制入口（hooks 已全部调用，安全 return）
  const onCopy = async (e: React.MouseEvent): Promise<void> => {
    e.stopPropagation(); // 折叠卡 header 上点复制不触发展开 / 折叠
    if (await copyText(text)) setCopied(true);
  };
  return (
    <Tooltip title={copied ? t('sessionTimeline.copied') : t('sessionTimeline.copy')}>
      <IconButton
        className="xk-copy-btn"
        size="small"
        onClick={(e) => void onCopy(e)}
        aria-label={t('sessionTimeline.copy')}
        sx={sx}
      >
        {copied ? <CheckRoundedIcon fontSize="inherit" /> : <ContentCopyRoundedIcon fontSize="inherit" />}
      </IconButton>
    </Tooltip>
  );
}

// 可折叠卡片（思考 / 工具调用 / 工具结果共用）。自带展开态，是纯展示子组件。
// tone 给图标上强调色：工具调用=accent、失败结果=danger，否则继承标题灰（避免一排卡片成"无差别横杠"）。
function CollapsibleBlock(props: {
  icon: React.ReactNode;
  label: string;
  summary?: string;
  bodyText: string;
  italic?: boolean;
  defaultOpen?: boolean;
  tone?: 'accent' | 'danger';
}) {
  const theme = useTheme();
  const styles = sessionTimelineStyles(theme);
  const fp = theme.palette.foundation;
  const [open, setOpen] = useState(props.defaultOpen ?? false);
  const toneColor =
    props.tone === 'danger' ? fp.status.danger : props.tone === 'accent' ? fp.accent : undefined;
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
        <CopyButton text={props.bodyText} sx={styles.copyBtnInline} />
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
export const SessionTimeline = ({ sessionId, visible, onSend, onRawInput }: SessionTimelineProps) => {
  const theme = useTheme();
  const styles = sessionTimelineStyles(theme);
  const t = useT();
  const { events, title, supported, pending } = useSessionTimeline({ sessionId, active: visible });
  const scrollRef = useRef<HTMLDivElement>(null);
  const [text, setText] = useState('');

  // 新事件到达 / 待答卡出现 / 重新可见 → 贴底（监控场景默认看最新）。
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [events.length, visible, pending]);

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
            <Box sx={styles.userBubble}>
              {ev.text}
              <CopyButton text={ev.text ?? ''} sx={styles.copyBtnFloat} />
            </Box>
          </Box>
        );
      case 'assistant':
        return (
          <Box key={key} sx={styles.assistantBubble}>
            {ev.text}
            <CopyButton text={ev.text ?? ''} sx={styles.copyBtnFloat} />
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
        // AskUserQuestion：转录里的是【已答】记录 → 渲染成只读历史卡。待答态不在转录里，走 pending
        // （PreToolUse hook 旁路事件，见 scroll 末尾的交互卡），故转录这条恒只读、不接 onRawInput。
        if (ev.toolName === 'AskUserQuestion' && ev.questions && ev.questions.length > 0) {
          return (
            <AskQuestionCard key={key} questions={ev.questions} interactive={false} onRawInput={() => {}} />
          );
        }
        return (
          <CollapsibleBlock
            key={key}
            icon={<BuildRoundedIcon fontSize="small" />}
            label={ev.toolName || t('sessionTimeline.tool')}
            summary={summarize(ev)}
            bodyText={prettyInput(ev)}
            tone="accent"
          />
        );
      case 'tool_result':
        return (
          <CollapsibleBlock
            key={key}
            icon={
              ev.isError ? (
                <ErrorOutlineRoundedIcon fontSize="small" />
              ) : (
                <SubdirectoryArrowRightRoundedIcon fontSize="small" />
              )
            }
            label={t('sessionTimeline.result')}
            bodyText={ev.text ?? ''}
            tone={ev.isError ? 'danger' : undefined}
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

      {events.length === 0 && !pending ? (
        <Box sx={styles.empty}>{t('sessionTimeline.empty')}</Box>
      ) : (
        <Box ref={scrollRef} sx={styles.scroll}>
          {events.map(renderEvent)}
          {/* 待答 AskUserQuestion（hook 旁路推送、落盘前即到）→ 唯一可作答的交互卡，置于时间线末尾。 */}
          {pending && pending.length > 0 ? (
            <AskQuestionCard questions={pending} interactive onRawInput={onRawInput ?? (() => {})} />
          ) : null}
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
