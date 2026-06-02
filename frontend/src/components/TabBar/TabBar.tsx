import { Box, ListItemIcon, ListItemText, Menu, MenuItem, Tooltip, useTheme } from '@mui/material';
import ChatRounded from '@mui/icons-material/ChatRounded';
import TerminalRounded from '@mui/icons-material/TerminalRounded';
import CloseRounded from '@mui/icons-material/CloseRounded';
import DriveFileRenameOutlineRounded from '@mui/icons-material/DriveFileRenameOutlineRounded';
import PushPinRounded from '@mui/icons-material/PushPinRounded';
import { SortableContext, horizontalListSortingStrategy, useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useState } from 'react';
import type { CSSProperties, MouseEvent } from 'react';
import { TextInputDialog } from '@/components/TextInputDialog';
import { useT } from '@/i18n';
import type { Tab, TabViewMode } from '@/terminal';
import { tabBarStyles, type TabBarSx } from './TabBar.styles';
import { useTabBar } from './useTabBar';

export interface TabBarProps {
  /** 所属面板 id —— 拖拽时随 Tab 携带，供 DndPaneProvider 区分同面板重排 / 跨面板移动。 */
  paneId: string;
  tabs: Tab[];
  activeTabId: string;
  onSelect: (tabId: string) => void;
  onClose: (tabId: string) => void;
  onTogglePin: (tabId: string) => void;
  onRename: (tabId: string, title: string) => void;
  /** 活动 Tab 的视图模式（终端 / 卡片）。 */
  viewMode: TabViewMode;
  /** 活动 Tab 是否支持卡片视图（claude/reclaude 才有结构化转录）；false 时不渲染切换控件。 */
  canCard: boolean;
  /** 切换活动 Tab 的视图模式。 */
  onViewModeChange: (mode: TabViewMode) => void;
}

// 单个可拖拽标签：外层为 @dnd-kit sortable 节点（承载 transform），内层为视觉 tab（选择/关闭/置顶图标/右键菜单）。
// 拆成子组件是因为 useSortable 是 hook，不能在 map 回调里调用（忠实对应 cc-pane SortableTab）。
function SortableTab({
  tab,
  active,
  paneId,
  styles,
  closeLabel,
  onSelect,
  onClose,
  onContextMenu,
  registerTabNode,
}: {
  tab: Tab;
  active: boolean;
  paneId: string;
  styles: TabBarSx;
  closeLabel: string;
  onSelect: (tabId: string) => void;
  onClose: (tabId: string) => void;
  onContextMenu: (event: MouseEvent, tabId: string) => void;
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
      onContextMenu={(e) => onContextMenu(e, tab.id)}
    >
      <Box
        sx={[styles.tab, active ? styles.tabActive : styles.tabInactive]}
        role="tab"
        aria-selected={active}
        onClick={() => onSelect(tab.id)}
      >
        {tab.pinned && <PushPinRounded sx={styles.pinIcon} />}
        <Box component="span" sx={styles.title}>
          {tab.title}
        </Box>
        {!tab.pinned && (
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
        )}
      </Box>
    </Box>
  );
}

// 标签条：选择 / 关闭 / 新建 + 活动态 + @dnd-kit 拖拽（同面板 reorder / 跨面板 move / 拖边缘分屏）
// + 右键终端菜单（重命名 / 置顶 / 关闭）。置顶 Tab 显示 pin 图标且隐藏关闭按钮。
// DndContext 在更高层（DndPaneProvider）；本条只提供每面板的 SortableContext。
export const TabBar = ({
  paneId,
  tabs,
  activeTabId,
  onSelect,
  onClose,
  onTogglePin,
  onRename,
  viewMode,
  canCard,
  onViewModeChange,
}: TabBarProps) => {
  const theme = useTheme();
  const styles = tabBarStyles(theme);
  const t = useT();
  const { scrollRef, registerTabNode } = useTabBar(activeTabId, tabs.length);
  const closeLabel = t('terminal.tab.close');

  // 右键菜单锚定到鼠标位置（anchorPosition）；重命名走 TextInputDialog。
  const [menu, setMenu] = useState<{ tabId: string; mouseX: number; mouseY: number } | null>(null);
  const [renamingTabId, setRenamingTabId] = useState<string | null>(null);

  const menuTab = menu ? (tabs.find((tab) => tab.id === menu.tabId) ?? null) : null;
  const renamingTab = renamingTabId
    ? (tabs.find((tab) => tab.id === renamingTabId) ?? null)
    : null;

  const openMenu = (event: MouseEvent, tabId: string): void => {
    event.preventDefault();
    setMenu({ tabId, mouseX: event.clientX, mouseY: event.clientY });
  };
  const closeMenu = (): void => setMenu(null);

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
              onContextMenu={openMenu}
              registerTabNode={registerTabNode}
            />
          ))}
        </SortableContext>
      </Box>
      {canCard && (
        <Box sx={styles.viewToggle} role="group" aria-label={t('terminal.view.group')}>
          <Tooltip title={t('terminal.view.terminal')}>
            <Box
              sx={[styles.viewBtn, viewMode === 'terminal' ? styles.viewBtnActive : styles.viewBtnIdle]}
              role="button"
              aria-label={t('terminal.view.terminal')}
              aria-pressed={viewMode === 'terminal'}
              onClick={() => onViewModeChange('terminal')}
            >
              <TerminalRounded sx={{ fontSize: 15 }} />
              <Box component="span" sx={styles.viewBtnLabel}>
                {t('terminal.view.terminal')}
              </Box>
            </Box>
          </Tooltip>
          <Tooltip title={t('terminal.view.card')}>
            <Box
              sx={[styles.viewBtn, viewMode === 'card' ? styles.viewBtnActive : styles.viewBtnIdle]}
              role="button"
              aria-label={t('terminal.view.card')}
              aria-pressed={viewMode === 'card'}
              onClick={() => onViewModeChange('card')}
            >
              <ChatRounded sx={{ fontSize: 15 }} />
              <Box component="span" sx={styles.viewBtnLabel}>
                {t('terminal.view.card')}
              </Box>
            </Box>
          </Tooltip>
        </Box>
      )}

      <Menu
        open={menu !== null}
        onClose={closeMenu}
        anchorReference="anchorPosition"
        anchorPosition={menu ? { top: menu.mouseY, left: menu.mouseX } : undefined}
      >
        <MenuItem
          onClick={() => {
            if (menu) setRenamingTabId(menu.tabId);
            closeMenu();
          }}
        >
          <ListItemIcon>
            <DriveFileRenameOutlineRounded fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t('terminal.tab.rename')}</ListItemText>
        </MenuItem>
        <MenuItem
          onClick={() => {
            if (menu) onTogglePin(menu.tabId);
            closeMenu();
          }}
        >
          <ListItemIcon>
            <PushPinRounded fontSize="small" />
          </ListItemIcon>
          <ListItemText>{menuTab?.pinned ? t('terminal.tab.unpin') : t('terminal.tab.pin')}</ListItemText>
        </MenuItem>
        {!menuTab?.pinned && (
          <MenuItem
            onClick={() => {
              if (menu) onClose(menu.tabId);
              closeMenu();
            }}
          >
            <ListItemIcon>
              <CloseRounded fontSize="small" />
            </ListItemIcon>
            <ListItemText>{t('terminal.tab.close')}</ListItemText>
          </MenuItem>
        )}
      </Menu>

      {renamingTab && (
        <TextInputDialog
          open
          title={t('terminal.tab.renameTitle')}
          label={t('terminal.tab.renamePlaceholder')}
          okLabel={t('common.save')}
          cancelLabel={t('common.cancel')}
          initialValue={renamingTab.title}
          onSubmit={(value) => {
            onRename(renamingTab.id, value);
            setRenamingTabId(null);
          }}
          onClose={() => setRenamingTabId(null)}
        />
      )}
    </Box>
  );
};
