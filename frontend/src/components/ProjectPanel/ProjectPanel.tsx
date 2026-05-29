import { Box, IconButton, Tooltip, useTheme } from '@mui/material';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import { useT } from '@/i18n';
import type { Project } from '@/services';
import { projectPanelStyles } from './ProjectPanel.styles';
import { useProjectPanel } from './useProjectPanel';

// ProjectPanel：扁平项目登记表面板（终端页左列）。列项目 / 添加（原生选目录）/
// 点项目启动终端（onLaunchProject）/ 移除（带确认）。改名·别名编辑、选 CLI 留 3b。
export interface ProjectPanelProps {
  onLaunchProject: (project: Project) => void;
}

const displayName = (p: Project): string => p.alias ?? p.name;

export const ProjectPanel = ({ onLaunchProject }: ProjectPanelProps) => {
  const theme = useTheme();
  const styles = projectPanelStyles(theme);
  const t = useT();
  const { projects, loading, add, remove } = useProjectPanel();

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
            onClick={() => onLaunchProject(p)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                onLaunchProject(p);
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
    </Box>
  );
};
