import { lazy, Suspense } from 'react';
import { Box, IconButton, Tooltip, Typography, useTheme } from '@mui/material';
import AccountTreeRoundedIcon from '@mui/icons-material/AccountTreeRounded';
import HistoryRoundedIcon from '@mui/icons-material/HistoryRounded';
import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import { useT } from '@/i18n';
import { useCurrentDir } from '@/currentdir';
import { rightPanelDrawerStyles } from './RightPanelDrawer.styles';
import { useRightPanel } from './RightPanelProvider';

// Git / 本地历史页内容做成右侧抽屉 —— 复用整页组件，传 boundPath 进「绑定模式」（隐藏页头 + 项目选择器，
// 改由抽屉头展示当前目录）。lazy 拆包：两页都较重（DiffView 等），不进主 bundle，按需加载。
const GitPanel = lazy(async () => ({ default: (await import('@/pages/GitPage/GitPage')).GitPage }));
const HistoryPanel = lazy(async () => ({ default: (await import('@/pages/HistoryPage/HistoryPage')).HistoryPage }));

// 路径末段（抽屉头展示当前绑定目录的短名）。
function basename(path: string): string {
  const norm = path.replace(/[\\/]+$/, '');
  const idx = Math.max(norm.lastIndexOf('/'), norm.lastIndexOf('\\'));
  return idx >= 0 ? norm.slice(idx + 1) : norm;
}

// 右侧抽屉宿主：active 为空不渲染（不占布局宽度）。绑定目标 = 当前聚焦目录（回落到树根）。
export const RightPanelDrawer = () => {
  const theme = useTheme();
  const styles = rightPanelDrawerStyles(theme);
  const t = useT();
  const { active, close } = useRightPanel();
  const { currentDir, rootDir } = useCurrentDir();

  if (!active) return null;

  const boundPath = currentDir ?? rootDir;
  const isGit = active === 'git';
  const Icon = isGit ? AccountTreeRoundedIcon : HistoryRoundedIcon;
  const title = isGit ? t('route.git') : t('route.history');

  return (
    <Box component="aside" sx={styles.root} aria-label={title}>
      <Box sx={styles.header}>
        <Icon fontSize="small" sx={styles.headerIcon} />
        <Typography component="span" sx={styles.title}>
          {title}
        </Typography>
        {boundPath && (
          <Typography component="span" sx={styles.path} title={boundPath}>
            {basename(boundPath)}
          </Typography>
        )}
        <Tooltip title={t('rightPanel.close')}>
          <IconButton size="small" aria-label={t('rightPanel.close')} onClick={close}>
            <CloseRoundedIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      <Box sx={styles.body}>
        <Suspense fallback={<Box sx={styles.loading}>{t('common.loading')}</Box>}>
          {isGit ? <GitPanel boundPath={boundPath} /> : <HistoryPanel boundPath={boundPath} />}
        </Suspense>
      </Box>
    </Box>
  );
};
