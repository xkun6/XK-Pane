import type { ReactNode } from 'react';
import { Box, Chip, IconButton, Tooltip, Typography, useTheme } from '@mui/material';
import { keyframes } from '@mui/system';
import CheckCircleRoundedIcon from '@mui/icons-material/CheckCircleRounded';
import DonutLargeRoundedIcon from '@mui/icons-material/DonutLargeRounded';
import RadioButtonUncheckedRoundedIcon from '@mui/icons-material/RadioButtonUncheckedRounded';
import WbSunnyRoundedIcon from '@mui/icons-material/WbSunnyRounded';
import FlagRoundedIcon from '@mui/icons-material/FlagRounded';
import CheckBoxRoundedIcon from '@mui/icons-material/CheckBoxRounded';
import CalendarTodayRoundedIcon from '@mui/icons-material/CalendarTodayRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import { useT } from '@/i18n';
import { BUILTIN_TODO_TYPES } from '../useTodoPage';
import type { Todo } from '@/services';
import { todoPageStyles } from '../TodoPage.styles';

// in_progress 状态图标缓慢自转（对齐 cc-pane animate-[spin_4s_linear_infinite]）；
// prefers-reduced-motion 时停转（无障碍）。
const spin = keyframes`to { transform: rotate(360deg); }`;

interface Props {
  todo: Todo;
  selected: boolean;
  showMyDay?: boolean;
  onSelect: (todo: Todo) => void;
  onToggleStatus: (todo: Todo) => void;
  onToggleMyDay: (todo: Todo) => void;
  onDelete: (todo: Todo) => void;
  onDispatch?: (todo: Todo) => void;
}

export const TodoListItem = ({
  todo,
  selected,
  showMyDay = true,
  onSelect,
  onToggleStatus,
  onToggleMyDay,
  onDelete,
  onDispatch,
}: Props) => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const t = useT();

  const priorityColor =
    todo.priority === 'high'
      ? theme.palette.error.main
      : todo.priority === 'medium'
        ? theme.palette.warning.main
        : theme.palette.foundation.text.secondary;

  const isOverdue =
    !!todo.dueDate && todo.status !== 'done' && new Date(todo.dueDate).getTime() < Date.now();

  const statusIcon = (): ReactNode => {
    if (todo.status === 'done') return <CheckCircleRoundedIcon fontSize="small" sx={{ color: theme.palette.success.main }} />;
    if (todo.status === 'in_progress')
      return (
        <DonutLargeRoundedIcon
          fontSize="small"
          sx={{
            color: theme.palette.info.main,
            animation: `${spin} 4s linear infinite`,
            '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
          }}
        />
      );
    return <RadioButtonUncheckedRoundedIcon fontSize="small" sx={{ color: theme.palette.foundation.text.secondary }} />;
  };

  const typeLabel = (BUILTIN_TODO_TYPES as readonly string[]).includes(todo.todoType)
    ? t(`todo.type.${todo.todoType}`)
    : todo.todoType;

  const doneSubs = todo.subtasks.filter((s) => s.completed).length;
  const stop = (e: React.MouseEvent): void => e.stopPropagation();

  return (
    <Box sx={selected ? { ...styles.item, ...styles.itemSelected } : styles.item} onClick={() => onSelect(todo)}>
      <Box sx={styles.itemMain}>
        <Tooltip title={t('todo.item.statusTip')}>
          <IconButton
            size="small"
            sx={styles.statusBtn}
            onClick={(e) => {
              stop(e);
              onToggleStatus(todo);
            }}
            aria-label={t('todo.item.statusTip')}
          >
            {statusIcon()}
          </IconButton>
        </Tooltip>

        <Typography sx={todo.status === 'done' ? styles.itemTitleDone : styles.itemTitle}>
          {todo.title}
        </Typography>

        {showMyDay && (
          <IconButton
            size="small"
            className={todo.myDay ? undefined : 'todo-hover-action'}
            onClick={(e) => {
              stop(e);
              onToggleMyDay(todo);
            }}
            aria-label={t('todo.item.myDayTip')}
          >
            <WbSunnyRoundedIcon
              fontSize="small"
              sx={{ color: todo.myDay ? theme.palette.warning.main : 'inherit' }}
            />
          </IconButton>
        )}

        <FlagRoundedIcon fontSize="small" sx={{ color: priorityColor }} />

        {onDispatch && (
          <IconButton
            size="small"
            className="todo-hover-action"
            onClick={(e) => {
              stop(e);
              onDispatch(todo);
            }}
            aria-label={t('todo.dispatch.tip')}
          >
            <PlayArrowRoundedIcon fontSize="small" />
          </IconButton>
        )}

        <IconButton
          size="small"
          className="todo-hover-action"
          onClick={(e) => {
            stop(e);
            onDelete(todo);
          }}
          aria-label={t('todo.delete.title')}
        >
          <DeleteOutlineRoundedIcon fontSize="small" />
        </IconButton>
      </Box>

      {(todo.todoType || todo.tags.length > 0 || todo.subtasks.length > 0 || todo.dueDate) && (
        <Box sx={styles.itemMeta}>
          {todo.todoType && (
            <Chip size="small" variant="outlined" label={typeLabel} sx={styles.metaChip} />
          )}
          {todo.tags.slice(0, 3).map((tag) => (
            <Chip key={tag} size="small" label={tag} sx={styles.metaChip} />
          ))}
          {todo.tags.length > 3 && <Box component="span" sx={styles.metaText}>{`+${todo.tags.length - 3}`}</Box>}
          {todo.subtasks.length > 0 && (
            <Box component="span" sx={styles.metaText}>
              <CheckBoxRoundedIcon sx={{ fontSize: 14 }} />
              {t('todo.item.subtasks', { done: String(doneSubs), total: String(todo.subtasks.length) })}
            </Box>
          )}
          {todo.dueDate && (
            <Box component="span" sx={isOverdue ? styles.metaOverdue : styles.metaText}>
              <CalendarTodayRoundedIcon sx={{ fontSize: 13 }} />
              {new Date(todo.dueDate).toLocaleDateString()}
            </Box>
          )}
        </Box>
      )}
    </Box>
  );
};
