import { useEffect, useMemo, useRef, useState } from 'react';
import { Box, IconButton, TextField, Tooltip, Typography, useTheme } from '@mui/material';
import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import PsychologyRoundedIcon from '@mui/icons-material/PsychologyRounded';
import SubdirectoryArrowRightRoundedIcon from '@mui/icons-material/SubdirectoryArrowRightRounded';
import ErrorOutlineRoundedIcon from '@mui/icons-material/ErrorOutlineRounded';
import SendRoundedIcon from '@mui/icons-material/SendRounded';
import { useT } from '@/i18n';
import type { TimelineEvent } from '@/services';
import { sessionTimelineStyles } from './SessionTimeline.styles';
import { useSessionTimeline } from './useSessionTimeline';
import { AskQuestionCard } from './AskQuestionCard';
import { CopyButton } from './messages/CopyButton';
import { CollapsibleBlock } from './messages/CollapsibleBlock';
import { DiffCard } from './messages/DiffCard';
import { BashCard } from './messages/BashCard';
import { TodoCard } from './messages/TodoCard';
import { PlanCard } from './messages/PlanCard';
import { getToolMeta, parseToolInput, summarizeToolInput } from './messages/toolMeta';

export interface SessionTimelineProps {
  sessionId: string;
  visible: boolean; // false=终端态：不渲染覆盖层、不轮询（保持挂载、状态不丢）
  onSend: (text: string) => void; // 把整句写进 PTY（父组件拼 '\r' 提交给 Agent）
  onRawInput?: (data: string) => void; // 原样写入 PTY（不追加 '\r'）：AskUserQuestion 作答按键序列
}

// 工具调用正文（折叠展开后的完整输入）：缩进 JSON，解析失败回退原串。
function prettyInput(ev: TimelineEvent): string {
  if (!ev.toolInput) return '';
  try {
    return JSON.stringify(JSON.parse(ev.toolInput), null, 2);
  } catch {
    return ev.toolInput;
  }
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

  // toolUseId → toolName：给工具结果卡补上来源工具名（结果块本身不带工具名，靠对应 tool_use 关联）。
  const toolNameById = useMemo(() => {
    const m = new Map<string, string>();
    for (const ev of events) {
      if (ev.kind === 'tool_use' && ev.toolUseId && ev.toolName) m.set(ev.toolUseId, ev.toolName);
    }
    return m;
  }, [events]);

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

  // 工具调用卡：AskUserQuestion → 只读问题卡；TodoWrite → 清单卡；ExitPlanMode → 计划卡；
  // 其余 → 带工具图标/色调的折叠卡（header 摘要挑最有信息量字段，正文是完整输入 JSON）。
  const renderToolUse = (ev: TimelineEvent, key: string): React.ReactNode => {
    if (ev.toolName === 'AskUserQuestion' && ev.questions && ev.questions.length > 0) {
      // 转录里的是【已答】记录 → 只读历史卡。待答态走 pending（hook 旁路，见末尾交互卡）。
      return <AskQuestionCard key={key} questions={ev.questions} interactive={false} onRawInput={() => {}} />;
    }
    const input = parseToolInput(ev.toolInput);
    const todos = input && Array.isArray(input.todos) ? input.todos : null;
    if (ev.toolName === 'TodoWrite' && todos && todos.length > 0) {
      return <TodoCard key={key} input={input} timestamp={ev.timestamp} />; // 有 todos 才用清单卡，否则回退折叠卡
    }
    if (ev.toolName === 'ExitPlanMode' && typeof input?.plan === 'string') {
      return <PlanCard key={key} input={input} timestamp={ev.timestamp} />;
    }
    const { Icon, tone } = getToolMeta(ev.toolName);
    return (
      <CollapsibleBlock
        key={key}
        icon={<Icon fontSize="small" />}
        label={ev.toolName || t('sessionTimeline.tool')}
        summary={summarizeToolInput(ev.toolName, input)}
        bodyText={prettyInput(ev)}
        tone={tone}
        timestamp={ev.timestamp}
      />
    );
  };

  // 工具结果卡：Edit 结构化差异 → diff 卡；Bash → 输出分离卡；其余 → 折叠卡（label 取来源工具名、
  // 折叠态露前 6 行预览、失败标 danger）。
  const renderToolResult = (ev: TimelineEvent, key: string): React.ReactNode => {
    if (ev.detail?.kind === 'edit') {
      return <DiffCard key={key} detail={ev.detail} timestamp={ev.timestamp} />;
    }
    const toolName = ev.toolUseId ? toolNameById.get(ev.toolUseId) : undefined;
    if (ev.detail?.kind === 'bash') {
      return <BashCard key={key} detail={ev.detail} label={toolName ?? 'Bash'} timestamp={ev.timestamp} />;
    }
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
        label={toolName ?? t('sessionTimeline.result')}
        bodyText={ev.text ?? ''}
        tone={ev.isError ? 'danger' : undefined}
        timestamp={ev.timestamp}
        previewLines={6}
      />
    );
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
            previewLines={4}
            timestamp={ev.timestamp}
          />
        );
      case 'tool_use':
        return renderToolUse(ev, key);
      case 'tool_result':
        return renderToolResult(ev, key);
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
