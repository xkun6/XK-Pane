import { useState } from 'react';
import { Box, IconButton } from '@mui/material';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import PublicRoundedIcon from '@mui/icons-material/PublicRounded';
import ChevronRightRoundedIcon from '@mui/icons-material/ChevronRightRounded';
import { useSortable } from '@dnd-kit/sortable';
import type { WorkspaceProject } from '@/services';
import type { WorkspaceTreeSx } from './WorkspaceTree.styles';
import type { MenuAnchor } from './WorkspaceContextMenu';
import { SessionSubList } from './SessionSubList';
import { projectDisplayName, projectSubtitle } from './projectDisplay';

export interface WorkspaceProjectItemProps {
  project: WorkspaceProject;
  styles: WorkspaceTreeSx;
  sessionsToggleLabel: string;
  onOpenMenu: (anchor: MenuAnchor) => void;
}

// 树第二层：工作空间下的项目节点。行首 chevron 展开「会话子列表」（按需扫盘，移植 cc-pane）；
// 行体左键 / 右键 / Enter 弹菜单（启动 + 编辑入口）。整行可拖拽（@dnd-kit，限同一工作空间内排序）。
export const WorkspaceProjectItem = ({
  project,
  styles,
  sessionsToggleLabel,
  onOpenMenu,
}: WorkspaceProjectItemProps) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: project.id,
  });
  const [sessionsOpen, setSessionsOpen] = useState(false);
  // SSH 远程项目：路径是合成键、会话扫盘按本地路径无意义 → 不展开本地会话子列表，行首占位对齐。
  const isSSH = project.ssh != null;
  // 垂直列表：只取 Y 轴位移，锁定 X=0（否则指针水平移动会把节点推出，触发横向滚动条）。
  const style = {
    transform: transform ? `translate3d(0px, ${transform.y}px, 0)` : undefined,
    transition,
    opacity: isDragging ? 0.4 : undefined,
  };

  const open = (e: { clientX: number; clientY: number }): void =>
    onOpenMenu({ top: e.clientY, left: e.clientX });

  return (
    <Box ref={setNodeRef} style={style}>
      <Box
        sx={styles.projectRow}
        title={projectSubtitle(project)}
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
        {isSSH ? (
          <Box sx={styles.projectChevron} />
        ) : (
          <IconButton
            size="small"
            sx={styles.projectChevron}
            aria-label={sessionsToggleLabel}
            aria-expanded={sessionsOpen}
            onPointerDown={(e) => e.stopPropagation()}
            onClick={(e) => {
              e.stopPropagation();
              setSessionsOpen((v) => !v);
            }}
          >
            <ChevronRightRoundedIcon
              fontSize="small"
              sx={{ transform: sessionsOpen ? 'rotate(90deg)' : 'none', transition: 'transform 120ms ease' }}
            />
          </IconButton>
        )}
        {isSSH ? (
          <PublicRoundedIcon fontSize="small" sx={styles.projectIcon} />
        ) : (
          <FolderRoundedIcon fontSize="small" sx={styles.projectIcon} />
        )}
        <Box sx={styles.projectText}>
          <Box sx={styles.projectName}>{projectDisplayName(project)}</Box>
          <Box sx={styles.projectPath}>{projectSubtitle(project)}</Box>
        </Box>
      </Box>

      {!isSSH && sessionsOpen && (
        <SessionSubList
          target={{ id: project.id, name: projectDisplayName(project), path: project.path }}
          styles={styles}
        />
      )}
    </Box>
  );
};
