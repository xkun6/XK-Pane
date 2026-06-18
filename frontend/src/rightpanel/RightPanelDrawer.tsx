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

// 把绝对文件路径换算为「项目内正斜杠相对路径」（= 本地历史 store 键，对齐
// watcher 的 ToSlash(Rel(root, abs))）。仅作比较用归一（D7：实际文件 IO 仍用原始路径），
// 切片保留原始大小写，仅前缀判断不区分大小写（Windows）。文件不在根内 → null。
function relativeKey(abs: string | null, root: string | null): string | null {
  if (!abs || !root) return null;
  const a = abs.replace(/\\/g, '/');
  const r = root.replace(/\\/g, '/').replace(/\/+$/, '');
  if (!a.toLowerCase().startsWith(r.toLowerCase() + '/')) return null;
  return a.slice(r.length + 1);
}

// 右侧抽屉宿主：active 为空不渲染（不占布局宽度）。绑定目标 = 当前聚焦目录（回落到树根）。
export const RightPanelDrawer = () => {
  const theme = useTheme();
  const styles = rightPanelDrawerStyles(theme);
  const t = useT();
  const { active, close } = useRightPanel();
  const { currentDir, rootDir, currentFile } = useCurrentDir();

  if (!active) return null;

  const isGit = active === 'git';
  // Git 走 currentDir（gitx 用 DetectDotGit 向上找仓库根，子目录也正确）；
  // 本地历史按「项目根」精确存储、不向上查找，必须绑 rootDir（文件页打开的项目根）——
  // 否则进子目录会查到空历史、在子目录错建 .xkpanes、且 live 刷新（事件按项目根过滤）失效。
  const boundPath = isGit ? currentDir ?? rootDir : rootDir ?? currentDir;
  // 本地历史抽屉：当前打开的文件相对项目根 → 自动跟随其时间线（Git 抽屉不需要）。
  const boundFile = isGit ? null : relativeKey(currentFile, boundPath);
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
          {isGit ? <GitPanel boundPath={boundPath} /> : <HistoryPanel boundPath={boundPath} boundFile={boundFile} />}
        </Suspense>
      </Box>
    </Box>
  );
};
