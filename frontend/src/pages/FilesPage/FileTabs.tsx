import { useEffect, useRef } from 'react';
import { Box, IconButton, useTheme } from '@mui/material';
import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import { useT } from '@/i18n';
import { filesPageStyles } from './FilesPage.styles';
import type { UseFilesPageResult } from './useFilesPage';

const baseName = (p: string): string => p.replace(/\\/g, '/').split('/').filter(Boolean).pop() ?? p;
const parentName = (p: string): string => {
  const parts = p.replace(/\\/g, '/').split('/').filter(Boolean);
  return parts.length >= 2 ? (parts[parts.length - 2] ?? '') : '';
};

// 顶部多文件标签条（VS Code 式）：单击激活 / 双击固定预览 tab / 关闭按钮或鼠标中键关闭。
// 脏 tab 默认显示圆点，悬停换成关闭按钮；同名文件在文件名后追加压暗父目录名消歧。
export const FileTabs = ({ vm }: { vm: UseFilesPageResult }) => {
  const theme = useTheme();
  const styles = filesPageStyles(theme);
  const t = useT();
  const tabRefs = useRef<Map<string, HTMLDivElement>>(new Map());

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

  return (
    <Box
      role="tablist"
      aria-label={t('files.tabs.ariaLabel')}
      sx={styles.tabsBar}
      onWheel={(e) => {
        e.currentTarget.scrollLeft += e.deltaY;
      }}
    >
      {vm.tabs.map((tb) => {
        const active = tb.path === vm.openFilePath;
        const dirty = tb.editValue !== tb.originalValue;
        const name = baseName(tb.path);
        const ambiguous = (nameCounts.get(name) ?? 0) > 1;
        return (
          <Box
            key={tb.path}
            ref={(el: HTMLDivElement | null) => {
              if (el) tabRefs.current.set(tb.path, el);
              else tabRefs.current.delete(tb.path);
            }}
            role="tab"
            aria-selected={active}
            title={tb.path}
            sx={active ? styles.tabActive : styles.tab}
            onClick={() => vm.activateTab(tb.path)}
            onDoubleClick={() => vm.pinTab(tb.path)}
            onAuxClick={(e) => {
              if (e.button === 1) void vm.closeTab(tb.path);
            }}
            onMouseDown={(e) => {
              if (e.button === 1) e.preventDefault();
            }}
          >
            <Box component="span" sx={tb.preview ? styles.tabNamePreview : styles.tabName}>
              {name}
            </Box>
            {ambiguous && (
              <Box component="span" sx={styles.tabDirSuffix}>
                {` · ${parentName(tb.path)}`}
              </Box>
            )}
            <Box sx={styles.tabRightSlot}>
              {dirty && <Box className="tabDirtyDot" sx={styles.tabDirtyDot} />}
              <IconButton
                size="small"
                className="tabClose"
                sx={styles.tabCloseBtn}
                aria-label={t('files.tabs.close')}
                onClick={(e) => {
                  e.stopPropagation();
                  void vm.closeTab(tb.path);
                }}
              >
                <CloseRoundedIcon sx={{ fontSize: 14 }} />
              </IconButton>
            </Box>
          </Box>
        );
      })}
    </Box>
  );
};
