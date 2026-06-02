import { Box, useTheme } from '@mui/material';
import DragIndicatorRoundedIcon from '@mui/icons-material/DragIndicatorRounded';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Todo } from '@/services';
import { todoPageStyles } from '../TodoPage.styles';
import { TodoListItem } from './TodoListItem';

interface Props {
  todo: Todo;
  selected: boolean;
  onSelect: (t: Todo) => void;
  onToggleStatus: (t: Todo) => void;
  onToggleMyDay: (t: Todo) => void;
  onDelete: (t: Todo) => void;
  onDispatch?: (t: Todo) => void;
}

// 平铺模式下的可拖拽行：左侧拖拽手柄（hover 显）+ TodoListItem。对齐 cc-pane SortableTodoListItem。
export const SortableTodoListItem = (p: Props) => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: p.todo.id,
  });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <Box ref={setNodeRef} style={style} sx={styles.sortableRow}>
      <Box className="todo-grip" sx={styles.grip} {...attributes} {...listeners}>
        <DragIndicatorRoundedIcon fontSize="small" />
      </Box>
      <Box sx={styles.gripFlex}>
        <TodoListItem
          todo={p.todo}
          selected={p.selected}
          onSelect={p.onSelect}
          onToggleStatus={p.onToggleStatus}
          onToggleMyDay={p.onToggleMyDay}
          onDelete={p.onDelete}
          onDispatch={p.onDispatch}
        />
      </Box>
    </Box>
  );
};
