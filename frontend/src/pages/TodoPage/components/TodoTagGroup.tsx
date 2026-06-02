import { useState } from 'react';
import { Box, Button, Chip, Collapse, useTheme } from '@mui/material';
import ChevronRightRoundedIcon from '@mui/icons-material/ChevronRightRounded';
import type { Todo } from '@/services';
import { todoPageStyles } from '../TodoPage.styles';
import { TodoListItem } from './TodoListItem';

interface Props {
  label: string;
  todos: Todo[];
  selectedId: string | null;
  onSelect: (t: Todo) => void;
  onToggleStatus: (t: Todo) => void;
  onToggleMyDay: (t: Todo) => void;
  onDelete: (t: Todo) => void;
  onDispatch?: (t: Todo) => void;
}

// 分组折叠容器（groupMode !== none 时用）。对齐 cc-pane TodoTagGroup：可折叠，头部 = 分组名 + done/total。
// 分组模式禁用拖拽（不包 Sortable）。
export const TodoTagGroup = (p: Props) => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const [open, setOpen] = useState(true);
  const doneCount = p.todos.filter((t) => t.status === 'done').length;

  return (
    <Box>
      <Button disableRipple onClick={() => setOpen(!open)} sx={styles.groupHeader}>
        <ChevronRightRoundedIcon
          sx={{ ...styles.groupChevron, transform: open ? 'rotate(90deg)' : 'none' }}
        />
        <Chip size="small" label={p.label} sx={styles.metaChip} />
        <Box component="span" sx={styles.groupCount}>{`${doneCount}/${p.todos.length}`}</Box>
      </Button>
      <Collapse in={open}>
        <Box sx={styles.groupBody}>
          {p.todos.map((todo) => (
            <TodoListItem
              key={todo.id}
              todo={todo}
              selected={p.selectedId === todo.id}
              showMyDay={false}
              onSelect={p.onSelect}
              onToggleStatus={p.onToggleStatus}
              onToggleMyDay={p.onToggleMyDay}
              onDelete={p.onDelete}
              onDispatch={p.onDispatch}
            />
          ))}
        </Box>
      </Collapse>
    </Box>
  );
};
