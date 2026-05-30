import { useState } from 'react';
import { Box, Button, Chip, IconButton, TextField, Tooltip, Typography, useTheme } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import CheckCircleRoundedIcon from '@mui/icons-material/CheckCircleRounded';
import RadioButtonUncheckedRoundedIcon from '@mui/icons-material/RadioButtonUncheckedRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import { useT } from '@/i18n';
import type { Todo } from '@/services';
import { todoPageStyles } from './TodoPage.styles';
import { useTodoPage } from './useTodoPage';

// TodoPage：自包含的待办功能（不依赖终端栈），用于端到端验证 Wails 服务往返。
// 列表 + 新建 + 状态勾选 + 删除（原生确认）+ 子任务（增 / 勾 / 删）。

export const TodoPage = () => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const t = useT();
  const vm = useTodoPage();

  const errorTitle = t('todo.error.title');

  const handleRemove = (todo: Todo): void => {
    void vm.remove(todo, {
      confirmTitle: t('todo.removeConfirm.title'),
      confirmMessage: t('todo.removeConfirm.message', { title: todo.title }),
      okLabel: t('todo.removeConfirm.ok'),
      cancelLabel: t('todo.removeConfirm.cancel'),
      errorTitle,
    });
  };

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Typography sx={styles.eyebrow}>{t('todo.eyebrow')}</Typography>
        <Typography sx={styles.title}>{t('todo.title')}</Typography>
      </Box>

      {vm.stats && vm.stats.total > 0 && (
        <Box sx={styles.statsRow}>
          <Chip
            size="small"
            variant="outlined"
            sx={styles.priorityChip}
            label={t('todo.stats.total', { count: String(vm.stats.total) })}
          />
          <Chip
            size="small"
            variant="outlined"
            sx={styles.priorityChip}
            label={t('todo.stats.todo', { count: String(vm.stats.byStatus['todo'] ?? 0) })}
          />
          <Chip
            size="small"
            variant="outlined"
            sx={styles.priorityChip}
            label={t('todo.stats.inProgress', { count: String(vm.stats.byStatus['in_progress'] ?? 0) })}
          />
          <Chip
            size="small"
            variant="outlined"
            sx={styles.priorityChip}
            label={t('todo.stats.done', { count: String(vm.stats.byStatus['done'] ?? 0) })}
          />
          {vm.stats.overdue > 0 && (
            <Chip
              size="small"
              variant="outlined"
              sx={{
                ...styles.priorityChip,
                color: theme.palette.foundation.status.danger,
                borderColor: theme.palette.foundation.status.danger,
              }}
              label={t('todo.stats.overdue', { count: String(vm.stats.overdue) })}
            />
          )}
        </Box>
      )}

      <Box sx={styles.addRow}>
        <TextField
          size="small"
          fullWidth
          placeholder={t('todo.addPlaceholder')}
          value={vm.newTitle}
          onChange={(e) => vm.setNewTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void vm.submitNew(errorTitle);
          }}
        />
        <Button
          variant="contained"
          startIcon={<AddRoundedIcon />}
          onClick={() => void vm.submitNew(errorTitle)}
          sx={{ textTransform: 'none', whiteSpace: 'nowrap' }}
        >
          {t('todo.add')}
        </Button>
      </Box>

      <Box sx={styles.list}>
        {!vm.loading && vm.todos.length === 0 && <Box sx={styles.empty}>{t('todo.empty')}</Box>}
        {vm.todos.map((todo) => (
          <TodoRow
            key={todo.id}
            todo={todo}
            styles={styles}
            t={t}
            errorTitle={errorTitle}
            onToggleStatus={() => void vm.toggleStatus(todo, errorTitle)}
            onRemove={() => handleRemove(todo)}
            onAddSubtask={(title) => void vm.addSubtask(todo.id, title, errorTitle)}
            onToggleSubtask={(id, completed) => void vm.toggleSubtask(id, completed, errorTitle)}
            onRemoveSubtask={(id) => void vm.removeSubtask(id, errorTitle)}
          />
        ))}
      </Box>
    </Box>
  );
};

interface TodoRowProps {
  todo: Todo;
  styles: ReturnType<typeof todoPageStyles>;
  t: ReturnType<typeof useT>;
  errorTitle: string;
  onToggleStatus: () => void;
  onRemove: () => void;
  onAddSubtask: (title: string) => void;
  onToggleSubtask: (id: string, completed: boolean) => void;
  onRemoveSubtask: (id: string) => void;
}

const PRIORITY_COLOR: Record<string, 'status.danger' | 'status.warning' | 'text.secondary'> = {
  high: 'status.danger',
  medium: 'status.warning',
  low: 'text.secondary',
};

// TodoRow 持有「新增子任务」的临时输入态（纯视图态，非业务/跨会话状态，留在组件内）。
const TodoRow = ({
  todo,
  styles,
  t,
  onToggleStatus,
  onRemove,
  onAddSubtask,
  onToggleSubtask,
  onRemoveSubtask,
}: TodoRowProps) => {
  const theme = useTheme();
  const [subDraft, setSubDraft] = useState('');
  const done = todo.status === 'done';

  const submitSub = (): void => {
    if (!subDraft.trim()) return;
    onAddSubtask(subDraft);
    setSubDraft('');
  };

  const prioToken = PRIORITY_COLOR[todo.priority] ?? 'text.secondary';
  const [prioGroup, prioKey] = prioToken.split('.') as ['status' | 'text', string];
  const prioColor =
    prioGroup === 'status'
      ? theme.palette.foundation.status[prioKey as 'danger' | 'warning' | 'success']
      : theme.palette.foundation.text[prioKey as 'secondary'];

  return (
    <Box sx={styles.row}>
      <Box sx={styles.rowMain}>
        <Tooltip title={done ? t('todo.actions.markUndone') : t('todo.actions.markDone')}>
          <IconButton
            size="small"
            aria-label={done ? t('todo.actions.markUndone') : t('todo.actions.markDone')}
            onClick={onToggleStatus}
          >
            {done ? (
              <CheckCircleRoundedIcon fontSize="small" sx={{ color: theme.palette.foundation.status.success }} />
            ) : (
              <RadioButtonUncheckedRoundedIcon fontSize="small" />
            )}
          </IconButton>
        </Tooltip>
        <Typography sx={done ? styles.rowTitleDone : styles.rowTitle}>{todo.title}</Typography>
        <Chip
          label={t(`todo.priority.${todo.priority}`)}
          size="small"
          variant="outlined"
          sx={{ ...styles.priorityChip, color: prioColor, borderColor: prioColor }}
        />
        <Tooltip title={t('todo.actions.remove')}>
          <IconButton size="small" aria-label={t('todo.actions.remove')} onClick={onRemove}>
            <DeleteOutlineRoundedIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      {todo.subtasks.length > 0 && (
        <Box sx={styles.subList}>
          {todo.subtasks.map((sub) => (
            <Box key={sub.id} sx={styles.subRow}>
              <IconButton
                size="small"
                aria-label={sub.completed ? t('todo.actions.markUndone') : t('todo.actions.markDone')}
                onClick={() => onToggleSubtask(sub.id, !sub.completed)}
              >
                {sub.completed ? (
                  <CheckCircleRoundedIcon fontSize="small" sx={{ color: theme.palette.foundation.status.success }} />
                ) : (
                  <RadioButtonUncheckedRoundedIcon fontSize="small" />
                )}
              </IconButton>
              <Typography sx={sub.completed ? styles.subTitleDone : styles.subTitle}>{sub.title}</Typography>
              <IconButton
                size="small"
                aria-label={t('todo.actions.remove')}
                onClick={() => onRemoveSubtask(sub.id)}
              >
                <DeleteOutlineRoundedIcon fontSize="small" />
              </IconButton>
            </Box>
          ))}
        </Box>
      )}

      <Box sx={styles.subAddRow}>
        <TextField
          size="small"
          variant="standard"
          fullWidth
          placeholder={t('todo.addSubtaskPlaceholder')}
          value={subDraft}
          onChange={(e) => setSubDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') submitSub();
          }}
        />
        <Tooltip title={t('todo.actions.addSubtask')}>
          <IconButton size="small" aria-label={t('todo.actions.addSubtask')} onClick={submitSub}>
            <AddRoundedIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>
    </Box>
  );
};
