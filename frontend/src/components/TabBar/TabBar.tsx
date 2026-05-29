import { Box, useTheme } from '@mui/material';
import AddRounded from '@mui/icons-material/AddRounded';
import CloseRounded from '@mui/icons-material/CloseRounded';
import { useT } from '@/i18n';
import type { Tab } from '@/terminal';
import { tabBarStyles } from './TabBar.styles';
import { useTabBar } from './useTabBar';

export interface TabBarProps {
  tabs: Tab[];
  activeTabId: string;
  onSelect: (tabId: string) => void;
  onClose: (tabId: string) => void;
  onAdd: () => void;
}

// MVP 标签条：选择 / 关闭 / 新建 + 活动态样式。
// 延后（需 reducer.reorderTabs + 父级 DndContext）：@dnd-kit 拖拽重排；以及 pin / 重命名 / 右键菜单。
export const TabBar = ({ tabs, activeTabId, onSelect, onClose, onAdd }: TabBarProps) => {
  const theme = useTheme();
  const styles = tabBarStyles(theme);
  const t = useT();
  const { scrollRef, registerTabNode } = useTabBar(activeTabId, tabs.length);

  return (
    <Box sx={styles.bar}>
      <Box ref={scrollRef} sx={styles.items} role="tablist">
        {tabs.map((tab) => {
          const active = tab.id === activeTabId;
          return (
            <Box
              key={tab.id}
              ref={(node: HTMLDivElement | null) => registerTabNode(tab.id, node)}
              sx={[styles.tab, active ? styles.tabActive : styles.tabInactive]}
              role="tab"
              aria-selected={active}
              onClick={() => onSelect(tab.id)}
            >
              <Box component="span" sx={styles.title}>
                {tab.title}
              </Box>
              <Box
                className="tab-close"
                sx={styles.closeBtn}
                role="button"
                aria-label={t('terminal.tab.close')}
                onClick={(e) => {
                  e.stopPropagation();
                  onClose(tab.id);
                }}
              >
                <CloseRounded sx={{ fontSize: 14 }} />
              </Box>
            </Box>
          );
        })}
      </Box>
      <Box sx={styles.addBtn} role="button" aria-label={t('terminal.tab.new')} onClick={onAdd}>
        <AddRounded sx={{ fontSize: 18 }} />
      </Box>
    </Box>
  );
};
