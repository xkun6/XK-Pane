import { Box, IconButton, Tooltip } from '@mui/material';
import ChevronRightRoundedIcon from '@mui/icons-material/ChevronRightRounded';
import StarRoundedIcon from '@mui/icons-material/StarRounded';
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded';
import type { Workspace, WorkspaceProject } from '@/services';
import type { WorkspaceTreeSx } from './WorkspaceTree.styles';
import type { MenuAnchor } from './WorkspaceContextMenu';
import { WorkspaceProjectItem } from './WorkspaceProjectItem';

// 文案由父（持 t()）传入，遵 MVVM。projectCountAria 是带 count 的格式化函数。
export interface WorkspaceItemStrings {
  pin: string;
  unpin: string;
  expand: string;
  collapse: string;
  noProjects: string;
  projectCountAria: (count: number) => string;
}

export interface WorkspaceItemProps {
  workspace: Workspace;
  expanded: boolean;
  styles: WorkspaceTreeSx;
  strings: WorkspaceItemStrings;
  onToggleExpand: () => void;
  onTogglePinned: () => void;
  onOpenMenu: (anchor: MenuAnchor) => void;
  onOpenProjectMenu: (project: WorkspaceProject, anchor: MenuAnchor) => void;
}

const displayName = (w: Workspace): string => w.alias ?? w.name;

// 树第一层：工作空间节点。左键展开/折叠；右键弹菜单（启动 + 导入/重命名/别名/删除）。
// 行内常显收藏星（反映 pinned，点击 toggle）。展开时在下方渲染项目子列表。
export const WorkspaceItem = ({
  workspace,
  expanded,
  styles,
  strings,
  onToggleExpand,
  onTogglePinned,
  onOpenMenu,
  onOpenProjectMenu,
}: WorkspaceItemProps) => {
  const count = workspace.projects.length;

  return (
    <Box>
      <Box
        role="button"
        tabIndex={0}
        sx={styles.wsRow}
        aria-expanded={expanded}
        onClick={onToggleExpand}
        onContextMenu={(e) => {
          e.preventDefault();
          onOpenMenu({ top: e.clientY, left: e.clientX });
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            onToggleExpand();
          }
        }}
      >
        <ChevronRightRoundedIcon
          fontSize="small"
          sx={{ ...styles.chevron, transform: expanded ? 'rotate(90deg)' : 'none' }}
        />
        <Box sx={styles.wsName}>{displayName(workspace)}</Box>
        <Box component="span" sx={styles.countPill} aria-label={strings.projectCountAria(count)}>
          {count}
        </Box>
        <Tooltip title={workspace.pinned ? strings.unpin : strings.pin}>
          <IconButton
            size="small"
            sx={styles.starBtn}
            aria-label={workspace.pinned ? strings.unpin : strings.pin}
            onClick={(e) => {
              e.stopPropagation();
              onTogglePinned();
            }}
          >
            {workspace.pinned ? (
              <StarRoundedIcon fontSize="small" sx={styles.starOn} />
            ) : (
              <StarBorderRoundedIcon fontSize="small" />
            )}
          </IconButton>
        </Tooltip>
      </Box>

      {expanded && (
        <Box sx={styles.projectList}>
          {count === 0 && <Box sx={styles.projectEmpty}>{strings.noProjects}</Box>}
          {workspace.projects.map((p) => (
            <WorkspaceProjectItem
              key={p.id}
              project={p}
              styles={styles}
              onOpenMenu={(anchor) => onOpenProjectMenu(p, anchor)}
            />
          ))}
        </Box>
      )}
    </Box>
  );
};
