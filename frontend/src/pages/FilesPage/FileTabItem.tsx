import type { MouseEvent as ReactMouseEvent } from 'react';
import { Box, IconButton, type SxProps, type Theme } from '@mui/material';
import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';

interface Props {
  path: string;
  name: string;
  dirSuffix: string | null; // 同名文件消歧后缀（父目录名），无需消歧时为 null
  active: boolean;
  dirty: boolean;
  preview: boolean;
  styles: Record<string, SxProps<Theme>>;
  closeLabel: string;
  onRef: (el: HTMLDivElement | null) => void;
  onActivate: () => void;
  onClose: () => void;
  onContextMenu: (e: ReactMouseEvent<HTMLDivElement>) => void;
}

// 单个可拖拽排序的标签（VS Code 式）：单击激活（预览 tab 单击即固定）/ 中键关闭 / 右键菜单 / 拖拽排序。
// 拖拽整体绑在 tab 本体（无独立把手），靠 PointerSensor 的 activationConstraint 距离阈值
// 保证单击不会被误判成拖拽。
export const FileTabItem = (p: Props) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: p.path,
  });
  const dragStyle = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.6 : 1,
  };

  return (
    <Box
      ref={(el: HTMLDivElement | null) => {
        setNodeRef(el);
        p.onRef(el);
      }}
      style={dragStyle}
      {...attributes}
      {...listeners}
      role="tab"
      aria-selected={p.active}
      title={p.path}
      sx={p.active ? p.styles.tabActive : p.styles.tab}
      onClick={p.onActivate}
      onContextMenu={p.onContextMenu}
      onAuxClick={(e) => {
        if (e.button === 1) p.onClose();
      }}
      onMouseDown={(e) => {
        if (e.button === 1) e.preventDefault();
      }}
    >
      <Box component="span" sx={p.preview ? p.styles.tabNamePreview : p.styles.tabName}>
        {p.name}
      </Box>
      {p.dirSuffix !== null && (
        <Box component="span" sx={p.styles.tabDirSuffix}>
          {` · ${p.dirSuffix}`}
        </Box>
      )}
      <Box sx={p.styles.tabRightSlot}>
        {p.dirty && <Box className="tabDirtyDot" sx={p.styles.tabDirtyDot} />}
        <IconButton
          size="small"
          className="tabClose"
          sx={p.styles.tabCloseBtn}
          aria-label={p.closeLabel}
          onClick={(e) => {
            e.stopPropagation();
            p.onClose();
          }}
        >
          <CloseRoundedIcon sx={{ fontSize: 14 }} />
        </IconButton>
      </Box>
    </Box>
  );
};
