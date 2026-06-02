import { Box, useTheme } from '@mui/material';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import { TabBar } from '@/components/TabBar';
import { TerminalView } from '@/components/TerminalView';
import { useT } from '@/i18n';
import { useTerminal, type Panel } from '@/terminal';
import { EdgeDropZone } from './EdgeDropZone';
import { panelViewStyles } from './PaneTree.styles';

// 叶子面板：TabBar（其活动 Tab）+ 内容区（每 Tab 一个 TerminalView，活动可见、其余隐藏保挂载）。
// 无 Tab（初始 / 关掉最后会话）→ 渲染空态占位，不凭空开空 shell。
// 仅 claude/reclaude 会话有结构化转录 → 才提供卡片视图（与后端 transcriptsvc.GetTimeline 的 supported 同口径）。
const canShowCard = (cliTool?: string): boolean => cliTool === 'claude' || cliTool === 'reclaude';

export const PanelView = ({ panel }: { panel: Panel }) => {
  const theme = useTheme();
  const styles = panelViewStyles(theme);
  const t = useT();
  const { setActiveTab, closeTab, togglePin, renameTab, setTabViewMode } = useTerminal();

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

  const activeTab = panel.tabs.find((tab) => tab.id === panel.activeTabId);

  return (
    <Box sx={styles.panel}>
      <TabBar
        paneId={panel.id}
        tabs={panel.tabs}
        activeTabId={panel.activeTabId}
        onSelect={(tabId) => setActiveTab(panel.id, tabId)}
        onClose={(tabId) => closeTab(panel.id, tabId)}
        onTogglePin={(tabId) => togglePin(panel.id, tabId)}
        onRename={(tabId, title) => renameTab(panel.id, tabId, title)}
        viewMode={activeTab?.viewMode ?? 'terminal'}
        canCard={canShowCard(activeTab?.cliTool)}
        onViewModeChange={(mode) => {
          if (activeTab) setTabViewMode(activeTab.id, mode);
        }}
      />
      <Box sx={styles.content}>
        {panel.tabs.map((tab) => {
          const visible = tab.id === panel.activeTabId;
          return (
            <Box
              key={tab.id}
              sx={[styles.tabPane, visible ? styles.tabPaneActive : styles.tabPaneHidden]}
            >
              <TerminalView
                sessionId={tab.sessionId}
                active={visible}
                cliTool={tab.cliTool}
                viewMode={tab.viewMode ?? 'terminal'}
              />
            </Box>
          );
        })}
        {/* 四条边放置区：把任意 Tab 拖到本面板某边 → 在其旁拆出新面板（中心区则合并，见碰撞检测）。
            始终渲染，使本面板可作任意拖拽的目标；是否高亮/生效由 EdgeDropZone 按场景判定。 */}
        <EdgeDropZone paneId={panel.id} edge="left" paneTabCount={panel.tabs.length} />
        <EdgeDropZone paneId={panel.id} edge="right" paneTabCount={panel.tabs.length} />
        <EdgeDropZone paneId={panel.id} edge="top" paneTabCount={panel.tabs.length} />
        <EdgeDropZone paneId={panel.id} edge="bottom" paneTabCount={panel.tabs.length} />
      </Box>
    </Box>
  );
};
