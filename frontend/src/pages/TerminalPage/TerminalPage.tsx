import { Box, useTheme } from '@mui/material';
import { DndPaneProvider, PaneTree } from '@/components/PaneTree';
import { WorkspaceTree } from '@/components/WorkspaceTree';
import { useTerminal } from '@/terminal';
import { terminalPageStyles } from './TerminalPage.styles';

// 终端页：左列 Workspace 两层树（取代旧扁平 ProjectPanel）+ 右列递归 PaneTree
// （消费 TerminalContext.rootPane）。WorkspaceTree 自包含（消费 WorkspaceContext，
// CLI 启动在 P3a-5b 直接走 useTerminal().launchProject）。
export const TerminalPage = () => {
  const theme = useTheme();
  const styles = terminalPageStyles(theme);
  const { rootPane } = useTerminal();

  return (
    <Box sx={styles.root}>
      <WorkspaceTree />
      <Box sx={styles.body}>
        <DndPaneProvider>
          <PaneTree node={rootPane} />
        </DndPaneProvider>
      </Box>
    </Box>
  );
};
