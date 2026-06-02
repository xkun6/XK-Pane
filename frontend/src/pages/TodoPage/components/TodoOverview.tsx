import { useEffect } from 'react';
import { Box, Button, LinearProgress, Typography, useTheme } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import { useT } from '@/i18n';
import type { Todo, TodoStats } from '@/services';
import { todoPageStyles } from '../TodoPage.styles';

interface Props {
  stats: TodoStats | null;
  recent: Todo[];
  onSelectTodo: (todo: Todo) => void;
  onCreateNew: () => void;
  onLoadStats: () => void;
}

export const TodoOverview = ({ stats, recent, onSelectTodo, onCreateNew, onLoadStats }: Props) => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const t = useT();

  useEffect(() => {
    onLoadStats();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const total = stats?.total ?? 0;
  const byStatus = stats?.byStatus ?? {};
  const byPriority = stats?.byPriority ?? {};

  const card = (value: number, label: string, color?: string) => (
    <Box sx={styles.statCard}>
      <Typography sx={color ? { ...styles.statValue, color } : styles.statValue}>{value}</Typography>
      <Typography sx={styles.statLabel}>{label}</Typography>
    </Box>
  );

  const bar = (label: string, count: number, color: string) => {
    const pct = total > 0 ? (count / total) * 100 : 0;
    return (
      <Box sx={styles.barRow} key={label}>
        <Box sx={styles.barLabelRow}>
          <span>{label}</span>
          <span>{count}</span>
        </Box>
        <LinearProgress
          variant="determinate"
          value={pct}
          sx={{ borderRadius: 4, height: 6, '& .MuiLinearProgress-bar': { backgroundColor: color } }}
        />
      </Box>
    );
  };

  return (
    <Box sx={styles.rightPane}>
      <Box sx={styles.overviewBody}>
        <Box sx={styles.statGrid}>
          {card(total, t('todo.overview.total'))}
          {card(byStatus['todo'] ?? 0, t('todo.overview.todo'))}
          {card(byStatus['in_progress'] ?? 0, t('todo.overview.inProgress'), theme.palette.info.main)}
          {card(byStatus['done'] ?? 0, t('todo.overview.done'), theme.palette.success.main)}
          {card(stats?.overdue ?? 0, t('todo.overview.overdue'), theme.palette.error.main)}
        </Box>

        <Box>
          <Typography sx={styles.sectionHeading}>{t('todo.overview.priorityDist')}</Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {bar(t('todo.priority.high'), byPriority['high'] ?? 0, theme.palette.error.main)}
            {bar(t('todo.priority.medium'), byPriority['medium'] ?? 0, theme.palette.warning.main)}
            {bar(t('todo.priority.low'), byPriority['low'] ?? 0, theme.palette.foundation.text.secondary)}
          </Box>
        </Box>

        {recent.length > 0 && (
          <Box>
            <Typography sx={styles.sectionHeading}>{t('todo.overview.recent')}</Typography>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25 }}>
              {recent.map((todo) => (
                <Box key={todo.id} sx={styles.recentItem} onClick={() => onSelectTodo(todo)}>
                  <Typography sx={{ fontSize: 13 }} noWrap>
                    {todo.title}
                  </Typography>
                </Box>
              ))}
            </Box>
          </Box>
        )}

        <Button
          variant="outlined"
          startIcon={<AddRoundedIcon fontSize="small" />}
          onClick={onCreateNew}
          sx={{ borderRadius: 6, textTransform: 'none', alignSelf: 'flex-start' }}
        >
          {t('todo.overview.createNew')}
        </Button>
      </Box>
    </Box>
  );
};
