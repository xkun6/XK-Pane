import { Box } from '@mui/material';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { WorkspaceProject } from '@/services';
import type { WorkspaceTreeSx } from './WorkspaceTree.styles';
import type { MenuAnchor } from './WorkspaceContextMenu';

// 项目路径末段（兼容 / 与 \\，去尾分隔符）作为无别名时的显示名。
function basename(path: string): string {
  const norm = path.replace(/[\\/]+$/, '');
  const idx = Math.max(norm.lastIndexOf('/'), norm.lastIndexOf('\\'));
  return idx >= 0 ? norm.slice(idx + 1) : norm;
}

const displayName = (p: WorkspaceProject): string => p.alias ?? basename(p.path);

export interface WorkspaceProjectItemProps {
  project: WorkspaceProject;
  styles: WorkspaceTreeSx;
  onOpenMenu: (anchor: MenuAnchor) => void;
}

// 树第二层：工作空间下的项目节点。左键 / 右键 / Enter 弹菜单（启动 + 编辑入口）；
// 整行可拖拽（@dnd-kit，限同一工作空间内排序，靠 activationConstraint 距离区分点击与拖拽）。
export const WorkspaceProjectItem = ({ project, styles, onOpenMenu }: WorkspaceProjectItemProps) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: project.id,
  });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : undefined,
  };

  const open = (e: { clientX: number; clientY: number }): void =>
    onOpenMenu({ top: e.clientY, left: e.clientX });

  return (
    <Box
      ref={setNodeRef}
      style={style}
      sx={styles.projectRow}
      title={project.path}
      {...attributes}
      {...listeners}
      role="button"
      tabIndex={0}
      onClick={open}
      onContextMenu={(e) => {
        e.preventDefault();
        open(e);
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          const rect = e.currentTarget.getBoundingClientRect();
          onOpenMenu({ top: rect.bottom, left: rect.left });
        }
      }}
    >
      <FolderRoundedIcon fontSize="small" sx={styles.projectIcon} />
      <Box sx={styles.projectText}>
        <Box sx={styles.projectName}>{displayName(project)}</Box>
        <Box sx={styles.projectPath}>{project.path}</Box>
      </Box>
    </Box>
  );
};
