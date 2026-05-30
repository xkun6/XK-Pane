import { useState } from 'react';
import {
  Box,
  Divider,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Tooltip,
  useTheme,
} from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import SmartToyRoundedIcon from '@mui/icons-material/SmartToyRounded';
import { useT } from '@/i18n';
import { useCliTools } from '@/shared/hooks/useCliTools';
import type { Project } from '@/services';
import { projectPanelStyles } from './ProjectPanel.styles';
import { useProjectPanel } from './useProjectPanel';

// ProjectPanel：扁平项目登记表面板（终端页左列）。列项目 / 添加（原生选目录）/
// 点项目（或右键）弹启动菜单（打开终端 + 各 CLI 工具）/ 移除（带确认）。
// 启动选择经 onLaunchProject(project, cliTool) 上交给 TerminalPage 起会话。
export interface ProjectPanelProps {
  onLaunchProject: (project: Project, cliTool: string) => void;
}

const displayName = (p: Project): string => p.alias ?? p.name;

export const ProjectPanel = ({ onLaunchProject }: ProjectPanelProps) => {
  const theme = useTheme();
  const styles = projectPanelStyles(theme);
  const t = useT();
  const { projects, loading, add, remove } = useProjectPanel();
  const { tools, loading: toolsLoading } = useCliTools();

  // 启动菜单状态（纯 UI 态，留在 View）：锚到被点击的项目行。
  const [menu, setMenu] = useState<{ project: Project; anchorEl: HTMLElement } | null>(null);

  const openMenu = (project: Project, anchorEl: HTMLElement): void => {
    setMenu({ project, anchorEl });
  };
  const closeMenu = (): void => setMenu(null);
  const launch = (cliTool: string): void => {
    if (menu) onLaunchProject(menu.project, cliTool);
    closeMenu();
  };

  const handleAdd = (): void => {
    void add({
      dialogTitle: t('projectPanel.addDialogTitle'),
      errorTitle: t('projectPanel.error.title'),
    });
  };

  const handleRemove = (p: Project): void => {
    void remove(p, {
      confirmTitle: t('projectPanel.removeConfirm.title'),
      confirmMessage: t('projectPanel.removeConfirm.message', { name: displayName(p) }),
      okLabel: t('common.remove'),
      cancelLabel: t('common.cancel'),
      errorTitle: t('projectPanel.error.title'),
    });
  };

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Box component="span" sx={styles.title}>
          {t('projectPanel.title')}
        </Box>
        <Tooltip title={t('projectPanel.add')}>
          <IconButton size="small" aria-label={t('projectPanel.add')} onClick={handleAdd} sx={styles.addBtn}>
            <AddRoundedIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      <Box sx={styles.list}>
        {!loading && projects.length === 0 && <Box sx={styles.empty}>{t('projectPanel.empty')}</Box>}

        {projects.map((p) => (
          <Box
            key={p.id}
            role="button"
            tabIndex={0}
            sx={styles.row}
            title={p.path}
            onClick={(e) => openMenu(p, e.currentTarget)}
            onContextMenu={(e) => {
              e.preventDefault();
              openMenu(p, e.currentTarget);
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                openMenu(p, e.currentTarget);
              }
            }}
          >
            <FolderRoundedIcon fontSize="small" sx={styles.rowIcon} />
            <Box sx={styles.rowText}>
              <Box sx={styles.rowName}>{displayName(p)}</Box>
              <Box sx={styles.rowPath}>{p.path}</Box>
            </Box>
            <Tooltip title={t('projectPanel.remove')}>
              <IconButton
                className="xk-project-remove"
                size="small"
                aria-label={t('projectPanel.remove')}
                sx={styles.removeBtn}
                onClick={(e) => {
                  e.stopPropagation();
                  handleRemove(p);
                }}
              >
                <DeleteOutlineRoundedIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        ))}
      </Box>

      <Menu
        anchorEl={menu?.anchorEl ?? null}
        open={menu !== null}
        onClose={closeMenu}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
      >
        <MenuItem onClick={() => launch('none')}>
          <ListItemIcon>
            <TerminalRoundedIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t('projectPanel.launch.openTerminal')}</ListItemText>
        </MenuItem>
        <Divider />
        {toolsLoading && <MenuItem disabled>{t('projectPanel.launch.loading')}</MenuItem>}
        {!toolsLoading &&
          tools.map((tool) => (
            <MenuItem key={tool.id} disabled={!tool.installed} onClick={() => launch(tool.id)}>
              <ListItemIcon>
                <SmartToyRoundedIcon fontSize="small" />
              </ListItemIcon>
              <ListItemText
                primary={tool.displayName}
                secondary={
                  tool.installed
                    ? tool.version || undefined
                    : t('projectPanel.launch.notInstalled')
                }
              />
            </MenuItem>
          ))}
      </Menu>
    </Box>
  );
};
