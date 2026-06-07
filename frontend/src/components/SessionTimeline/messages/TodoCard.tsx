import { Box, Typography, useTheme } from '@mui/material';
import ChecklistRoundedIcon from '@mui/icons-material/ChecklistRounded';
import CheckCircleRoundedIcon from '@mui/icons-material/CheckCircleRounded';
import AutorenewRoundedIcon from '@mui/icons-material/AutorenewRounded';
import RadioButtonUncheckedRoundedIcon from '@mui/icons-material/RadioButtonUncheckedRounded';
import { useT } from '@/i18n';
import { sessionTimelineStyles } from '../SessionTimeline.styles';
import { formatTime } from './time';
import { mergeSx } from './sx';

interface TodoItem {
  content?: string;
  status?: string;
  activeForm?: string;
}

// TodoCard：把 TodoWrite 的 input.todos 渲染成带状态图标的清单（completed 勾 / in_progress 转圈 / pending 空圈）
// + 「已完成 x/y」。input 解析失败 / 无 todos 返回 null（调用方回退普通工具卡）。
export function TodoCard({
  input,
  timestamp,
}: {
  input: Record<string, unknown> | null;
  timestamp?: string;
}) {
  const theme = useTheme();
  const s = sessionTimelineStyles(theme);
  const t = useT();
  const todos = Array.isArray(input?.todos) ? (input?.todos as TodoItem[]) : [];
  if (todos.length === 0) return null;
  const done = todos.filter((td) => td.status === 'completed').length;
  const time = formatTime(timestamp);
  return (
    <Box sx={s.todoCard}>
      <Box sx={s.todoHeader}>
        <ChecklistRoundedIcon fontSize="small" />
        <Typography component="span" sx={s.todoHeaderText}>
          {t('sessionTimeline.todo.title')}
        </Typography>
        {time ? (
          <Box component="span" sx={s.timestamp}>
            {time}
          </Box>
        ) : null}
        <Box component="span" sx={s.todoCount}>
          {t('sessionTimeline.todo.done', { done: String(done), total: String(todos.length) })}
        </Box>
      </Box>
      <Box sx={s.todoList}>
        {todos.map((td, i) => {
          const status = td.status ?? 'pending';
          const { Icon, iconSx, label } =
            status === 'completed'
              ? { Icon: CheckCircleRoundedIcon, iconSx: s.todoStatusDone, label: t('sessionTimeline.todo.completed') }
              : status === 'in_progress'
                ? { Icon: AutorenewRoundedIcon, iconSx: s.todoStatusActive, label: t('sessionTimeline.todo.inProgress') }
                : { Icon: RadioButtonUncheckedRoundedIcon, iconSx: s.todoStatusPending, label: t('sessionTimeline.todo.pending') };
          return (
            <Box key={i} sx={s.todoRow}>
              <Icon sx={mergeSx(s.todoStatusIcon, iconSx)} titleAccess={label} />
              <Box sx={s.todoBody}>
                <Box sx={mergeSx(s.todoText, status === 'completed' && s.todoTextDone)}>{td.content}</Box>
                {status === 'in_progress' && td.activeForm ? (
                  <Box sx={s.todoActiveForm}>{td.activeForm}</Box>
                ) : null}
              </Box>
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}
