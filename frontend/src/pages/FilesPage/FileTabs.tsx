import { useEffect, useRef, useState } from 'react';
import { Box, Menu, MenuItem, useTheme } from '@mui/material';
import {
  DndContext,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import { SortableContext, horizontalListSortingStrategy } from '@dnd-kit/sortable';
import { useT } from '@/i18n';
import { filesPageStyles } from './FilesPage.styles';
import type { UseFilesPageResult } from './useFilesPage';
import { FileTabItem } from './FileTabItem';

const baseName = (p: string): string => p.replace(/\\/g, '/').split('/').filter(Boolean).pop() ?? p;
const parentName = (p: string): string => {
  const parts = p.replace(/\\/g, '/').split('/').filter(Boolean);
  return parts.length >= 2 ? (parts[parts.length - 2] ?? '') : '';
};

interface TabMenuState {
  mouseX: number;
  mouseY: number;
  path: string;
}

// 顶部多文件标签条（VS Code 式）：单击激活 / 双击固定预览 tab / 关闭按钮或鼠标中键关闭 /
// 右键菜单（关闭 / 关闭其它 / 关闭右侧 / 全部关闭）/ 拖拽排序。
// 脏 tab 默认显示圆点，悬停换成关闭按钮；同名文件在文件名后追加压暗父目录名消歧。
export const FileTabs = ({ vm }: { vm: UseFilesPageResult }) => {
  const theme = useTheme();
  const styles = filesPageStyles(theme);
  const t = useT();
  const tabRefs = useRef<Map<string, HTMLDivElement>>(new Map());
  const [menu, setMenu] = useState<TabMenuState | null>(null);

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }));

  // 激活 tab 变化时滚动使其可见（水平溢出的 tab 条）。
  useEffect(() => {
    if (!vm.openFilePath) return;
    tabRefs.current.get(vm.openFilePath)?.scrollIntoView({ inline: 'nearest', block: 'nearest' });
  }, [vm.openFilePath]);

  if (vm.tabs.length === 0) return null;

  // 同名文件（不同目录）计数，用于决定是否显示消歧后缀。
  const nameCounts = new Map<string, number>();
  for (const tb of vm.tabs) {
    const name = baseName(tb.path);
    nameCounts.set(name, (nameCounts.get(name) ?? 0) + 1);
  }

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (over && active.id !== over.id) vm.reorderTabs(String(active.id), String(over.id));
  };

  const closeMenu = () => setMenu(null);
  const menuIdx = menu ? vm.tabs.findIndex((tb) => tb.path === menu.path) : -1;
  const isLastTab = menuIdx >= 0 && menuIdx === vm.tabs.length - 1;

  return (
    <>
      <Box
        role="tablist"
        aria-label={t('files.tabs.ariaLabel')}
        sx={styles.tabsBar}
        onWheel={(e) => {
          e.currentTarget.scrollLeft += e.deltaY;
        }}
      >
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={vm.tabs.map((tb) => tb.path)} strategy={horizontalListSortingStrategy}>
            {vm.tabs.map((tb) => {
              const active = tb.path === vm.openFilePath;
              const dirty = tb.editValue !== tb.originalValue;
              const name = baseName(tb.path);
              const ambiguous = (nameCounts.get(name) ?? 0) > 1;
              return (
                <FileTabItem
                  key={tb.path}
                  path={tb.path}
                  name={name}
                  dirSuffix={ambiguous ? parentName(tb.path) : null}
                  active={active}
                  dirty={dirty}
                  preview={tb.preview}
                  styles={styles}
                  closeLabel={t('files.tabs.close')}
                  onRef={(el) => {
                    if (el) tabRefs.current.set(tb.path, el);
                    else tabRefs.current.delete(tb.path);
                  }}
                  onActivate={() => vm.activateTab(tb.path)}
                  onPin={() => vm.pinTab(tb.path)}
                  onClose={() => void vm.closeTab(tb.path)}
                  onContextMenu={(e) => {
                    e.preventDefault();
                    setMenu({ mouseX: e.clientX, mouseY: e.clientY, path: tb.path });
                  }}
                />
              );
            })}
          </SortableContext>
        </DndContext>
      </Box>

      <Menu
        open={menu !== null}
        onClose={closeMenu}
        anchorReference="anchorPosition"
        anchorPosition={menu ? { top: menu.mouseY, left: menu.mouseX } : undefined}
      >
        <MenuItem
          onClick={() => {
            const path = menu?.path;
            closeMenu();
            if (path) void vm.closeTab(path);
          }}
        >
          {t('files.tabs.menu.close')}
        </MenuItem>
        <MenuItem
          disabled={vm.tabs.length === 1}
          onClick={() => {
            const path = menu?.path;
            closeMenu();
            if (path) void vm.closeOtherTabs(path);
          }}
        >
          {t('files.tabs.menu.closeOthers')}
        </MenuItem>
        <MenuItem
          disabled={isLastTab}
          onClick={() => {
            const path = menu?.path;
            closeMenu();
            if (path) void vm.closeTabsToRight(path);
          }}
        >
          {t('files.tabs.menu.closeRight')}
        </MenuItem>
        <MenuItem
          onClick={() => {
            closeMenu();
            void vm.closeAllTabs();
          }}
        >
          {t('files.tabs.menu.closeAll')}
        </MenuItem>
      </Menu>
    </>
  );
};
