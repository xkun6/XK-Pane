import { useState } from 'react';
import { Box, Drawer, IconButton, Typography, useMediaQuery, useTheme } from '@mui/material';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import { DndPaneProvider, PaneTree } from '@/components/PaneTree';
import { WorkspaceTree } from '@/components/WorkspaceTree';
import { WebWorkspaceTree } from '@/components/WebWorkspaceTree';
import { lifecycleRole } from '@/services/transport';
import { useT } from '@/i18n';
import { useTerminal } from '@/terminal';
import { terminalPageStyles } from './TerminalPage.styles';

// 终端页：左列工作区栏 + 右列递归 PaneTree（消费 TerminalContext.rootPane）。
// 左列按角色（P7 D1）：桌面 owner = WorkspaceTree；Web viewer = WebWorkspaceTree。
// 手机/窄屏（down('sm')）：左列固定 272px 会吃掉竖屏大半宽度 → 收进左侧抽屉，主体铺满，
// 顶部细栏放「工作区」开关，选中会话后抽屉自动收起（露出全宽终端/对话）。桌面/宽屏维持并排。
export const TerminalPage = () => {
  const theme = useTheme();
  const styles = terminalPageStyles(theme);
  const t = useT();
  const { rootPane } = useTerminal();
  const isMobile = useMediaQuery(theme.breakpoints.down('sm'));
  const [railOpen, setRailOpen] = useState(false);

  const rail =
    lifecycleRole === 'viewer' ? (
      <WebWorkspaceTree onSelect={isMobile ? () => setRailOpen(false) : undefined} />
    ) : (
      <WorkspaceTree />
    );

  const paneBody = (
    <Box sx={styles.body}>
      <DndPaneProvider>
        <PaneTree node={rootPane} />
      </DndPaneProvider>
    </Box>
  );

  if (isMobile) {
    return (
      <Box sx={styles.mobileRoot}>
        <Box sx={styles.mobileBar}>
          <IconButton
            size="small"
            aria-label={t('webWorkspaceTree.rail.title')}
            onClick={() => setRailOpen(true)}
          >
            <FolderRoundedIcon fontSize="small" />
          </IconButton>
          <Typography sx={styles.mobileBarLabel}>{t('webWorkspaceTree.rail.title')}</Typography>
        </Box>
        {paneBody}
        <Drawer
          anchor="left"
          open={railOpen}
          onClose={() => setRailOpen(false)}
          slotProps={{ paper: { sx: styles.drawerPaper } }}
        >
          {rail}
        </Drawer>
      </Box>
    );
  }

  return (
    <Box sx={styles.root}>
      {rail}
      {paneBody}
    </Box>
  );
};
