import { Box, Button, useTheme } from '@mui/material';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import { PaneTree } from '@/components/PaneTree';
import { useT } from '@/i18n';
import { useTerminal } from '@/terminal';
import { terminalPageStyles } from './TerminalPage.styles';
import { useTerminalPage } from './useTerminalPage';

// 终端页：顶部工具条（MVP 启动入口「打开文件夹」）+ 递归 PaneTree（消费 TerminalContext.rootPane）。
export const TerminalPage = () => {
  const theme = useTheme();
  const styles = terminalPageStyles(theme);
  const t = useT();
  const { rootPane } = useTerminal();
  const { openFolder } = useTerminalPage();

  return (
    <Box sx={styles.root}>
      <Box sx={styles.toolbar}>
        <Button
          size="small"
          variant="outlined"
          startIcon={<FolderOpenRoundedIcon />}
          onClick={() => void openFolder(t('terminalPage.openDialogTitle'))}
          sx={styles.openBtn}
        >
          {t('terminalPage.openFolder')}
        </Button>
      </Box>
      <Box sx={styles.body}>
        <PaneTree node={rootPane} />
      </Box>
    </Box>
  );
};
