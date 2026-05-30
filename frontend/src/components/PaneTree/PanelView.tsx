import { Box, useTheme } from '@mui/material';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import { TabBar } from '@/components/TabBar';
import { TerminalView } from '@/components/TerminalView';
import { useT } from '@/i18n';
import { useTerminal, type Panel } from '@/terminal';
import { panelViewStyles } from './PaneTree.styles';

// 叶子面板：TabBar（其活动 Tab）+ 内容区（每 Tab 一个 TerminalView，活动可见、其余隐藏保挂载）。
// 无 Tab（初始 / 关掉最后会话）→ 渲染空态占位，不凭空开空 shell。
export const PanelView = ({ panel }: { panel: Panel }) => {
  const theme = useTheme();
  const styles = panelViewStyles(theme);
  const t = useT();
  const { setActiveTab, closeTab, newTab } = useTerminal();

  if (panel.tabs.length === 0) {
    return (
      <Box sx={styles.panel}>
        <Box sx={styles.empty}>
          <TerminalRoundedIcon sx={styles.emptyIcon} />
          <Box sx={styles.emptyTitle}>{t('terminalPane.empty.title')}</Box>
          <Box sx={styles.emptyHint}>{t('terminalPane.empty.hint')}</Box>
        </Box>
      </Box>
    );
  }

  const handleAdd = () => {
    // MVP：以活动 Tab 的 project 派生新 Tab（无项目则空，待项目树/启动单元提供真实 projectPath）。
    const active = panel.tabs.find((t) => t.id === panel.activeTabId);
    newTab(panel.id, {
      projectId: active?.projectId ?? '',
      projectPath: active?.projectPath ?? '',
      cliTool: active?.cliTool,
      workspacePath: active?.workspacePath,
    });
  };

  return (
    <Box sx={styles.panel}>
      <TabBar
        tabs={panel.tabs}
        activeTabId={panel.activeTabId}
        onSelect={(tabId) => setActiveTab(panel.id, tabId)}
        onClose={(tabId) => closeTab(panel.id, tabId)}
        onAdd={handleAdd}
      />
      <Box sx={styles.content}>
        {panel.tabs.map((tab) => {
          const visible = tab.id === panel.activeTabId;
          return (
            <Box
              key={tab.id}
              sx={[styles.tabPane, visible ? styles.tabPaneActive : styles.tabPaneHidden]}
            >
              <TerminalView sessionId={tab.sessionId} active={visible} cliTool={tab.cliTool} />
            </Box>
          );
        })}
      </Box>
    </Box>
  );
};
