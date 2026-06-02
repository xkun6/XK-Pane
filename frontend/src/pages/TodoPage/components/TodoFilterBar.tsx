import { useState, type ReactNode } from 'react';
import { Box, Chip, IconButton, InputAdornment, Menu, MenuItem, TextField, Tooltip, useTheme } from '@mui/material';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import FormatListBulletedRoundedIcon from '@mui/icons-material/FormatListBulletedRounded';
import GridViewRoundedIcon from '@mui/icons-material/GridViewRounded';
import { useT } from '@/i18n';
import { todoPageStyles } from '../TodoPage.styles';
import { BUILTIN_TODO_TYPES, type GroupMode } from '../useTodoPage';

interface Props {
  searchText: string;
  onSearchChange: (v: string) => void;
  filterStatus: string | null;
  onStatusChange: (v: string | null) => void;
  filterPriority: string | null;
  onPriorityChange: (v: string | null) => void;
  filterType: string | null;
  onTypeChange: (v: string | null) => void;
  allTypes: string[];
  groupMode: GroupMode;
  onGroupModeChange: (m: GroupMode) => void;
}

const GROUP_MODES: GroupMode[] = ['none', 'tag', 'status', 'priority', 'scope'];

export const TodoFilterBar = ({
  searchText,
  onSearchChange,
  filterStatus,
  onStatusChange,
  filterPriority,
  onPriorityChange,
  filterType,
  onTypeChange,
  allTypes,
  groupMode,
  onGroupModeChange,
}: Props) => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const t = useT();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  const typeLabel = (v: string): string =>
    (BUILTIN_TODO_TYPES as readonly string[]).includes(v) ? t(`todo.type.${v}`) : v;

  const pillGroup = (
    options: { value: string | null; label: string }[],
    selected: string | null,
    onSelect: (v: string | null) => void,
  ): ReactNode => (
    <Box sx={styles.pillRow}>
      {options.map((o) => (
        <Chip
          key={o.value ?? '__all'}
          size="small"
          label={o.label}
          color={selected === o.value ? 'primary' : 'default'}
          variant={selected === o.value ? 'filled' : 'outlined'}
          onClick={() => onSelect(o.value)}
          sx={styles.pill}
        />
      ))}
    </Box>
  );

  const statusOptions = [
    { value: null, label: t('todo.filter.all') },
    { value: 'todo', label: t('todo.status.todo') },
    { value: 'in_progress', label: t('todo.status.in_progress') },
    { value: 'done', label: t('todo.status.done') },
  ];
  const priorityOptions = [
    { value: null, label: t('todo.filter.all') },
    { value: 'high', label: t('todo.priority.high') },
    { value: 'medium', label: t('todo.priority.medium') },
    { value: 'low', label: t('todo.priority.low') },
  ];
  const typeOptions = [
    { value: null, label: t('todo.filter.allTypes') },
    ...allTypes.map((ty) => ({ value: ty, label: typeLabel(ty) })),
  ];

  return (
    <Box sx={styles.filterBar}>
      <Box sx={styles.filterTopRow}>
        <TextField
          size="small"
          sx={styles.searchField}
          placeholder={t('todo.search.placeholder')}
          value={searchText}
          onChange={(e) => onSearchChange(e.target.value)}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <SearchRoundedIcon fontSize="small" />
                </InputAdornment>
              ),
            },
          }}
        />
        <Tooltip title={t('todo.filter.group.label')}>
          <IconButton
            size="small"
            color={groupMode !== 'none' ? 'primary' : 'default'}
            onClick={(e) => setAnchor(e.currentTarget)}
            aria-label={t('todo.filter.group.label')}
          >
            {groupMode !== 'none' ? (
              <GridViewRoundedIcon fontSize="small" />
            ) : (
              <FormatListBulletedRoundedIcon fontSize="small" />
            )}
          </IconButton>
        </Tooltip>
        <Menu anchorEl={anchor} open={Boolean(anchor)} onClose={() => setAnchor(null)}>
          {GROUP_MODES.map((m) => (
            <MenuItem
              key={m}
              selected={groupMode === m}
              onClick={() => {
                onGroupModeChange(m);
                setAnchor(null);
              }}
            >
              {t(`todo.filter.group.${m}`)}
            </MenuItem>
          ))}
        </Menu>
      </Box>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, overflowX: 'auto' }}>
        {pillGroup(statusOptions, filterStatus, onStatusChange)}
        <Box sx={styles.pillDivider} />
        {pillGroup(priorityOptions, filterPriority, onPriorityChange)}
        <Box sx={styles.pillDivider} />
        {pillGroup(typeOptions, filterType, onTypeChange)}
      </Box>
    </Box>
  );
};
