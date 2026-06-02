import { Box, Typography, useTheme } from '@mui/material';
import ChevronRightRoundedIcon from '@mui/icons-material/ChevronRightRounded';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import InsertDriveFileRoundedIcon from '@mui/icons-material/InsertDriveFileRounded';
import type { ReactNode } from 'react';
import { useT } from '@/i18n';
import type { FileEntry } from '@/services';
import { fileSystemTreeStyles } from './FileSystemTree.styles';
import { useFileSystemTree } from './useFileSystemTree';

export interface FileSystemTreeProps {
  rootPath: string;
  selectedPath: string | null;
  // 点击文件：选中并打开（驱动右侧编辑器）。
  onSelectFile: (entry: FileEntry) => void;
  // 点击目录行：把该目录设为当前上下文（驱动 Git/历史绑定）。展开/折叠同时发生。
  onActivateDir?: (path: string) => void;
}

// 懒加载文件系统目录树：根目录直接子项常驻，展开子目录时按需拉取并缓存。
// 目录在前、文件在后（后端已排序）。点目录设为当前上下文 + 展开，点文件打开。
export const FileSystemTree = ({
  rootPath,
  selectedPath,
  onSelectFile,
  onActivateDir,
}: FileSystemTreeProps) => {
  const theme = useTheme();
  const styles = fileSystemTreeStyles(theme);
  const t = useT();
  const vm = useFileSystemTree(rootPath);

  const handleRow = (entry: FileEntry): void => {
    if (entry.isDir) {
      vm.toggle(entry.path);
      onActivateDir?.(entry.path);
    } else {
      onSelectFile(entry);
    }
  };

  const renderNodes = (entries: FileEntry[], depth: number): ReactNode =>
    entries.map((entry) => {
      const isExpanded = vm.expanded.has(entry.path);
      const isSelected = entry.path === selectedPath;
      const kids = vm.childrenByPath[entry.path];
      return (
        <Box key={entry.path}>
          <Box
            sx={isSelected ? styles.rowActive : styles.row}
            style={{ paddingLeft: depth * 14 + 6 }}
            onClick={() => handleRow(entry)}
            role="treeitem"
            aria-selected={isSelected}
            aria-expanded={entry.isDir ? isExpanded : undefined}
          >
            {entry.isDir ? (
              <Box sx={styles.chevron} aria-hidden>
                {isExpanded ? (
                  <ExpandMoreRoundedIcon fontSize="small" />
                ) : (
                  <ChevronRightRoundedIcon fontSize="small" />
                )}
              </Box>
            ) : (
              <Box sx={styles.chevronSpacer} aria-hidden />
            )}
            <Box sx={entry.isDir ? styles.iconDir : styles.icon} aria-hidden>
              {entry.isDir ? (
                isExpanded ? (
                  <FolderOpenRoundedIcon fontSize="small" />
                ) : (
                  <FolderRoundedIcon fontSize="small" />
                )
              ) : (
                <InsertDriveFileRoundedIcon fontSize="small" />
              )}
            </Box>
            <Typography component="span" sx={styles.name} title={entry.name}>
              {entry.name}
            </Typography>
          </Box>
          {entry.isDir && isExpanded && (
            <>
              {vm.loading.has(entry.path) && (
                <Box sx={styles.hint} style={{ paddingLeft: (depth + 1) * 14 + 6 }}>
                  {t('files.tree.loading')}
                </Box>
              )}
              {vm.errored.has(entry.path) && (
                <Box sx={styles.errorHint} style={{ paddingLeft: (depth + 1) * 14 + 6 }}>
                  {t('files.tree.error')}
                </Box>
              )}
              {kids && renderNodes(kids, depth + 1)}
            </>
          )}
        </Box>
      );
    });

  return (
    <Box sx={styles.root} role="tree" aria-label={t('files.tree.ariaLabel')}>
      {vm.rootLoading && <Box sx={styles.hint}>{t('files.tree.loading')}</Box>}
      {vm.rootErrored && <Box sx={styles.errorHint}>{t('files.tree.error')}</Box>}
      {!vm.rootLoading && !vm.rootErrored && vm.rootEntries.length === 0 && (
        <Box sx={styles.hint}>{t('files.tree.empty')}</Box>
      )}
      {renderNodes(vm.rootEntries, 0)}
    </Box>
  );
};
