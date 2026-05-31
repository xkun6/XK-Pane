import { Box, useTheme } from '@mui/material';
import AddRounded from '@mui/icons-material/AddRounded';
import CloseRounded from '@mui/icons-material/CloseRounded';
import { SortableContext, horizontalListSortingStrategy, useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { CSSProperties } from 'react';
import { useT } from '@/i18n';
import type { Tab } from '@/terminal';
import { tabBarStyles, type TabBarSx } from './TabBar.styles';
import { useTabBar } from './useTabBar';

export interface TabBarProps {
  /** 所属面板 id —— 拖拽时随 Tab 携带，供 DndPaneProvider 区分同面板重排 / 跨面板移动。 */
  paneId: string;
  tabs: Tab[];
  activeTabId: string;
  onSelect: (tabId: string) => void;
  onClose: (tabId: string) => void;
  onAdd: () => void;
}

// 单个可拖拽标签：外层为 @dnd-kit sortable 节点（承载 transform），内层为视觉 tab（选择/关闭）。
// 拆成子组件是因为 useSortable 是 hook，不能在 map 回调里调用（忠实对应 cc-pane SortableTab）。
function SortableTab({
  tab,
  active,
  paneId,
  styles,
  closeLabel,
  onSelect,
  onClose,
  registerTabNode,
}: {
  tab: Tab;
  active: boolean;
  paneId: string;
  styles: TabBarSx;
  closeLabel: string;
  onSelect: (tabId: string) => void;
  onClose: (tabId: string) => void;
  registerTabNode: (tabId: string, node: HTMLDivElement | null) => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: tab.id,
    data: { type: 'tab', paneId, tab },
  });
  const style: CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : undefined,
  };

  return (
    <Box
      ref={(node: HTMLDivElement | null) => {
        setNodeRef(node);
        registerTabNode(tab.id, node);
      }}
      style={style}
      sx={styles.dragWrap}
      {...attributes}
      {...listeners}
    >
      <Box
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
          aria-label={closeLabel}
          onClick={(e) => {
            e.stopPropagation();
            onClose(tab.id);
          }}
        >
          <CloseRounded sx={{ fontSize: 14 }} />
        </Box>
      </Box>
    </Box>
  );
}

// 标签条：选择 / 关闭 / 新建 + 活动态 + @dnd-kit 拖拽重排（同面板 reorder / 跨面板 move）。
// 拖拽的 DndContext 在更高层（DndPaneProvider 包裹整棵 PaneTree）；本条只提供每面板的 SortableContext。
// 延后：pin / 重命名 / 右键菜单；拖到面板边缘分屏（P5b-2）。
export const TabBar = ({ paneId, tabs, activeTabId, onSelect, onClose, onAdd }: TabBarProps) => {
  const theme = useTheme();
  const styles = tabBarStyles(theme);
  const t = useT();
  const { scrollRef, registerTabNode } = useTabBar(activeTabId, tabs.length);
  const closeLabel = t('terminal.tab.close');

  return (
    <Box sx={styles.bar}>
      <Box ref={scrollRef} sx={styles.items} role="tablist">
        <SortableContext items={tabs.map((tab) => tab.id)} strategy={horizontalListSortingStrategy}>
          {tabs.map((tab) => (
            <SortableTab
              key={tab.id}
              tab={tab}
              active={tab.id === activeTabId}
              paneId={paneId}
              styles={styles}
              closeLabel={closeLabel}
              onSelect={onSelect}
              onClose={onClose}
              registerTabNode={registerTabNode}
            />
          ))}
        </SortableContext>
      </Box>
      <Box sx={styles.addBtn} role="button" aria-label={t('terminal.tab.new')} onClick={onAdd}>
        <AddRounded sx={{ fontSize: 18 }} />
      </Box>
    </Box>
  );
};
