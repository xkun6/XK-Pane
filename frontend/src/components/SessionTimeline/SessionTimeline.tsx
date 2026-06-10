import { useEffect, useMemo, useRef } from 'react';
import { Box, Typography, useTheme } from '@mui/material';
import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import PsychologyRoundedIcon from '@mui/icons-material/PsychologyRounded';
import SubdirectoryArrowRightRoundedIcon from '@mui/icons-material/SubdirectoryArrowRightRounded';
import ErrorOutlineRoundedIcon from '@mui/icons-material/ErrorOutlineRounded';
import { useT } from '@/i18n';
import { Composer } from '@/components/Composer';
import type { TimelineEvent } from '@/services';
import { sessionTimelineStyles } from './SessionTimeline.styles';
import { useSessionTimeline } from './useSessionTimeline';
import { AskQuestionCard } from './AskQuestionCard';
import { StatusLine } from './StatusLine';
import { CopyButton } from './messages/CopyButton';
import { CollapsibleBlock } from './messages/CollapsibleBlock';
import { DiffCard } from './messages/DiffCard';
import { BashCard } from './messages/BashCard';
import { TodoCard } from './messages/TodoCard';
import { PlanCard } from './messages/PlanCard';
import { ThinkingIndicator } from './messages/ThinkingIndicator';
import { getToolMeta, parseToolInput, summarizeToolInput } from './messages/toolMeta';

export interface SessionTimelineProps {
  sessionId: string;
  visible: boolean; // false=终端态：不渲染覆盖层、不轮询（保持挂载、状态不丢）
  onSend: (text: string) => Promise<void> | void; // 提交整句（父组件两段式写 PTY：文本→延时→'\r'）
  onRawInput?: (data: string) => void; // 原样写入 PTY（不追加 '\r'）：AskUserQuestion 作答按键序列
  onInterrupt?: () => void; // 打断当前运行（父组件写 Esc）；缺省不显示打断按钮
}

// 文件转储 / 网页类只读结果：折叠态不展开原文预览（噪声），只在 header 给行数摘要。
const RESULT_NO_PREVIEW = new Set(['Read', 'WebFetch', 'NotebookRead']);

// SessionTimeline：把活跃会话的 Claude 转录渲染成可重排的对话卡片（覆盖在 xterm 之上）。
// supported!==true（加载中 / 非 claude）时不渲染 → 露出底层 xterm 镜像（零干扰）。
export const SessionTimeline = ({ sessionId, visible, onSend, onRawInput, onInterrupt }: SessionTimelineProps) => {
  const theme = useTheme();
  const styles = sessionTimelineStyles(theme);
  const t = useT();
  const { events, title, supported, pending, busy, metrics } = useSessionTimeline({ sessionId, active: visible });
  const scrollRef = useRef<HTMLDivElement>(null);

  // toolUseId → toolName：给工具结果卡补上来源工具名（结果块本身不带工具名，靠对应 tool_use 关联）。
  const toolNameById = useMemo(() => {
    const m = new Map<string, string>();
    for (const ev of events) {
      if (ev.kind === 'tool_use' && ev.toolUseId && ev.toolName) m.set(ev.toolUseId, ev.toolName);
    }
    return m;
  }, [events]);

  // 自动贴底：仅当用户已接近底部（或刚切到卡片视图 / 待答卡刚出现）才贴底，避免用户上滚翻看历史时
  // 被 ~1.5s 轮询落的新事件强行拽回底部。
  const prevVisibleRef = useRef(visible);
  const prevPendingRef = useRef(pending != null);
  useEffect(() => {
    const becameVisible = visible && !prevVisibleRef.current;
    const pendingAppeared = pending != null && !prevPendingRef.current;
    prevVisibleRef.current = visible;
    prevPendingRef.current = pending != null;
    const el = scrollRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
    if (nearBottom || becameVisible || pendingAppeared) el.scrollTop = el.scrollHeight;
  }, [events.length, visible, pending, busy]);

  // 终端态（!visible）或无结构化转录（非 claude）→ 不渲染覆盖层、露出底层 xterm；但保持挂载，
  // hook 已累积的事件 / supported 不丢，切回卡片即时复现（无需从头重拉）。
  if (supported !== true || !visible) return null;

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
        bodyText={input ? JSON.stringify(input, null, 2) : (ev.toolInput ?? '')}
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
      return (
        <BashCard
          key={key}
          detail={ev.detail}
          label={toolName ?? t('sessionTimeline.tool')}
          isError={ev.isError}
          timestamp={ev.timestamp}
        />
      );
    }
    const text = ev.text ?? '';
    const lineCount = text ? text.split('\n').length : 0;
    // 折叠态摘要：多行给行数（一眼知体量）、单行直接显示内容片段（CSS 省略）。
    const summary = lineCount > 1 ? t('sessionTimeline.resultLines', { n: String(lineCount) }) : text.trim();
    const noPreview = toolName ? RESULT_NO_PREVIEW.has(toolName) : false;
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
        summary={summary}
        bodyText={text}
        tone={ev.isError ? 'danger' : undefined}
        timestamp={ev.timestamp}
        previewLines={noPreview ? undefined : 6}
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

      {events.length === 0 && !pending && !busy ? (
        <Box sx={styles.empty}>{t('sessionTimeline.empty')}</Box>
      ) : (
        <Box ref={scrollRef} sx={styles.scroll}>
          {events.map(renderEvent)}
          {/* 时间线末尾：待答优先（可作答交互卡），否则工作中显示「思考中…」liveness 指示器。 */}
          {pending && pending.length > 0 ? (
            <AskQuestionCard questions={pending} interactive onRawInput={onRawInput ?? (() => {})} />
          ) : busy ? (
            <ThinkingIndicator />
          ) : null}
        </Box>
      )}

      {/* 结构化输入条（paseo 式 Composer）：多行自适应 + 按会话草稿 + 两段式提交 + busy 态打断。 */}
      <Composer sessionId={sessionId} busy={busy} onSubmit={onSend} onInterrupt={onInterrupt} />

      {/* 输入框下方的状态行：上下文 / 用量(5h) / 本周（复用 claude-hud 产物，纯文字）。 */}
      <StatusLine metrics={metrics} />
    </Box>
  );
};
