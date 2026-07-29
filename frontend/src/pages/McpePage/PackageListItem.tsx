import { Box, IconButton, Tooltip } from '@mui/material';
import Inventory2RoundedIcon from '@mui/icons-material/Inventory2Rounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import { useSortable } from '@dnd-kit/sortable';
import type { MCPackage } from '@/services';
import type { mcpeStyles } from './McpePage.styles';

export interface PackageListItemProps {
  pkg: MCPackage;
  active: boolean;
  running: boolean;
  styles: ReturnType<typeof mcpeStyles>;
  deleteLabel: string;
  runningLabel: string;
  onSelect: () => void;
  onDelete: () => void;
}

// 包列表行：拆自 PackageList，挂 useSortable 支持 @dnd-kit 竖向拖拽排序（照抄 WorkspaceProjectItem）。
export const PackageListItem = ({
  pkg,
  active,
  running,
  styles,
  deleteLabel,
  runningLabel,
  onSelect,
  onDelete,
}: PackageListItemProps) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: pkg.id,
  });
  // 垂直列表：只取 Y 轴位移，锁定 X=0（否则指针水平移动会把节点推出，触发横向滚动条）。
  const style = {
    transform: transform ? `translate3d(0px, ${transform.y}px, 0)` : undefined,
    transition,
    opacity: isDragging ? 0.4 : undefined,
  };

  return (
    <Box ref={setNodeRef} style={style}>
      <Box
        sx={{ ...styles.row, ...(active ? styles.rowActive : {}) }}
        {...attributes}
        {...listeners}
        onClick={onSelect}
      >
        <Inventory2RoundedIcon sx={{ fontSize: 14, mr: 1, flexShrink: 0 }} />
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Box sx={styles.rowName}>{pkg.name}</Box>
          <Box sx={styles.rowPath}>{pkg.projectPath}</Box>
        </Box>
        {running && (
          <Box component="span" sx={styles.runBadge}>
            ● {runningLabel}
          </Box>
        )}
        <Tooltip title={deleteLabel}>
          <IconButton
            className="mcpe-row-del"
            size="small"
            sx={styles.rowDel}
            aria-label={deleteLabel}
            onPointerDown={(e) => e.stopPropagation()}
            onClick={(e) => {
              e.stopPropagation();
              onDelete();
            }}
          >
            <DeleteOutlineRoundedIcon sx={{ fontSize: 14 }} />
          </IconButton>
        </Tooltip>
      </Box>
    </Box>
  );
};
