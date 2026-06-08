import { Divider, ListItemIcon, ListItemText, Menu, MenuItem } from '@mui/material';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import SmartToyRoundedIcon from '@mui/icons-material/SmartToyRounded';
import CreateNewFolderRoundedIcon from '@mui/icons-material/CreateNewFolderRounded';
import DnsRoundedIcon from '@mui/icons-material/DnsRounded';
import DriveFileRenameOutlineRoundedIcon from '@mui/icons-material/DriveFileRenameOutlineRounded';
import LabelRoundedIcon from '@mui/icons-material/LabelRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import RemoveCircleOutlineRoundedIcon from '@mui/icons-material/RemoveCircleOutlineRounded';
import OpenInNewRoundedIcon from '@mui/icons-material/OpenInNewRounded';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import type { CliTool, Workspace, WorkspaceProject } from '@/services';

export interface MenuAnchor {
  top: number;
  left: number;
}

// 右键菜单目标：工作空间节点 or 项目节点（带鼠标锚点坐标）。
export type WorkspaceMenuState =
  | { kind: 'workspace'; workspace: Workspace; anchor: MenuAnchor }
  | { kind: 'project'; project: WorkspaceProject; anchor: MenuAnchor };

// 文案由父（持 t()）传入，遵 MVVM。
export interface WorkspaceMenuStrings {
  openTerminal: string;
  launchLoading: string;
  notInstalled: string;
  importProject: string;
  addSSHProject: string;
  rename: string;
  setAlias: string;
  delete: string;
  copyPath: string;
  copyName: string;
  removeProject: string;
  openInExplorer: string;
  openInFileBrowser: string;
}

export interface WorkspaceContextMenuProps {
  menu: WorkspaceMenuState | null;
  tools: CliTool[];
  toolsLoading: boolean;
  launchDisabled: boolean; // 工作空间未设根目录 → 启动项灰置
  strings: WorkspaceMenuStrings;
  onClose: () => void;
  onLaunch: (cliTool: string) => void;
  onImportProject: () => void;
  onAddSSHProject: () => void;
  onRename: () => void;
  onSetAlias: () => void;
  onDelete: () => void;
  onCopyPath: () => void;
  onCopyName: () => void;
  onRemoveProject: () => void;
  onOpenInExplorer: () => void;
  onOpenInFileBrowser: () => void;
}

// WorkspaceTree 的右键菜单：启动区（打开终端 + 各 CLI 工具，移植自 ProjectPanel）
// + 按节点类型的编辑区（工作空间：导入项目/重命名/别名/删除；项目：复制路径/别名/移除）。
export const WorkspaceContextMenu = ({
  menu,
  tools,
  toolsLoading,
  launchDisabled,
  strings,
  onClose,
  onLaunch,
  onImportProject,
  onAddSSHProject,
  onRename,
  onSetAlias,
  onDelete,
  onCopyPath,
  onCopyName,
  onRemoveProject,
  onOpenInExplorer,
  onOpenInFileBrowser,
}: WorkspaceContextMenuProps) => {
  // SSH 远程项目：本地文件管理器 / 文件浏览器入口无意义，按 ssh!=null 隐藏。
  const isSSHProject = menu?.kind === 'project' && menu.project.ssh != null;
  return (
    <Menu
      open={menu !== null}
      onClose={onClose}
      anchorReference="anchorPosition"
      anchorPosition={menu ? { top: menu.anchor.top, left: menu.anchor.left } : undefined}
    >
      <MenuItem disabled={launchDisabled} onClick={() => onLaunch('none')}>
        <ListItemIcon>
          <TerminalRoundedIcon fontSize="small" />
        </ListItemIcon>
        <ListItemText>{strings.openTerminal}</ListItemText>
      </MenuItem>

      {toolsLoading && <MenuItem disabled>{strings.launchLoading}</MenuItem>}
      {!toolsLoading &&
        tools.map((tool) => (
          <MenuItem
            key={tool.id}
            disabled={launchDisabled || !tool.installed}
            onClick={() => onLaunch(tool.id)}
          >
            <ListItemIcon>
              <SmartToyRoundedIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText
              primary={tool.displayName}
              secondary={tool.installed ? tool.version || undefined : strings.notInstalled}
            />
          </MenuItem>
        ))}

      <Divider />

      {menu?.kind === 'workspace' && [
        <MenuItem key="import" onClick={onImportProject}>
          <ListItemIcon>
            <CreateNewFolderRoundedIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{strings.importProject}</ListItemText>
        </MenuItem>,
        <MenuItem key="ssh" onClick={onAddSSHProject}>
          <ListItemIcon>
            <DnsRoundedIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{strings.addSSHProject}</ListItemText>
        </MenuItem>,
        <MenuItem key="rename" onClick={onRename}>
          <ListItemIcon>
            <DriveFileRenameOutlineRoundedIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{strings.rename}</ListItemText>
        </MenuItem>,
        <MenuItem key="alias" onClick={onSetAlias}>
          <ListItemIcon>
            <LabelRoundedIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{strings.setAlias}</ListItemText>
        </MenuItem>,
        <MenuItem key="delete" onClick={onDelete}>
          <ListItemIcon>
            <DeleteOutlineRoundedIcon fontSize="small" color="error" />
          </ListItemIcon>
          <ListItemText>{strings.delete}</ListItemText>
        </MenuItem>,
      ]}

      {menu?.kind === 'project' && [
        // 本地文件管理器 / 文件浏览器入口仅本地项目可用（SSH 远程项目隐藏，含其后的分隔符）。
        ...(!isSSHProject
          ? [
              <MenuItem key="reveal" onClick={onOpenInExplorer}>
                <ListItemIcon>
                  <OpenInNewRoundedIcon fontSize="small" />
                </ListItemIcon>
                <ListItemText>{strings.openInExplorer}</ListItemText>
              </MenuItem>,
              <MenuItem key="browse" onClick={onOpenInFileBrowser}>
                <ListItemIcon>
                  <FolderOpenRoundedIcon fontSize="small" />
                </ListItemIcon>
                <ListItemText>{strings.openInFileBrowser}</ListItemText>
              </MenuItem>,
              <Divider key="proj-div" />,
            ]
          : []),
        <MenuItem key="copy" onClick={onCopyPath}>
          <ListItemIcon>
            <ContentCopyRoundedIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{strings.copyPath}</ListItemText>
        </MenuItem>,
        <MenuItem key="copy-name" onClick={onCopyName}>
          <ListItemIcon>
            <ContentCopyRoundedIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{strings.copyName}</ListItemText>
        </MenuItem>,
        <MenuItem key="alias" onClick={onSetAlias}>
          <ListItemIcon>
            <LabelRoundedIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{strings.setAlias}</ListItemText>
        </MenuItem>,
        <MenuItem key="remove" onClick={onRemoveProject}>
          <ListItemIcon>
            <RemoveCircleOutlineRoundedIcon fontSize="small" color="error" />
          </ListItemIcon>
          <ListItemText>{strings.removeProject}</ListItemText>
        </MenuItem>,
      ]}
    </Menu>
  );
};
