import { Box, useTheme } from '@mui/material';
import { PaneTree } from '@/components/PaneTree';
import { ProjectPanel } from '@/components/ProjectPanel';
import { useTerminal } from '@/terminal';
import { terminalPageStyles } from './TerminalPage.styles';
import { useTerminalPage } from './useTerminalPage';

// 终端页：左列项目面板（列项目 / 添加 / 点击启动）+ 右列递归 PaneTree（消费 TerminalContext.rootPane）。
export const TerminalPage = () => {
  const theme = useTheme();
  const styles = terminalPageStyles(theme);
  const { rootPane } = useTerminal();
  const { launchProject } = useTerminalPage();

  return (
    <Box sx={styles.root}>
      <ProjectPanel onLaunchProject={launchProject} />
      <Box sx={styles.body}>
        <PaneTree node={rootPane} />
      </Box>
    </Box>
  );
};
