import {
  Box,
  Divider,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Typography,
  useTheme,
} from '@mui/material';
import ChevronRightRoundedIcon from '@mui/icons-material/ChevronRightRounded';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import InsertDriveFileRoundedIcon from '@mui/icons-material/InsertDriveFileRounded';
import CreateNewFolderRoundedIcon from '@mui/icons-material/CreateNewFolderRounded';
import NoteAddRoundedIcon from '@mui/icons-material/NoteAddRounded';
import DriveFileRenameOutlineRoundedIcon from '@mui/icons-material/DriveFileRenameOutlineRounded';
import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import LinkRoundedIcon from '@mui/icons-material/LinkRounded';
import OpenInNewRoundedIcon from '@mui/icons-material/OpenInNewRounded';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import { useState, type MouseEvent, type ReactNode } from 'react';
import { useT } from '@/i18n';
import { FileService, NativeDialogs, type FileEntry } from '@/services';
import { InputDialog } from '@/components/InputDialog';
import { fileSystemTreeStyles } from './FileSystemTree.styles';
import { useFileSystemTree, type FileTreeMutation } from './useFileSystemTree';

export interface FileSystemTreeProps {
  rootPath: string;
  selectedPath: string | null;
  // 点击文件：选中并打开（驱动右侧编辑器）。
  onSelectFile: (entry: FileEntry) => void;
  // 点击目录行：把该目录设为当前上下文（驱动 Git/历史绑定）。展开/折叠同时发生。
  onActivateDir?: (path: string) => void;
  // 文件树写操作（新建/改名/删除等）完成后的通知（上层据此同步编辑器选中态）。
  onAfterMutate?: (m: FileTreeMutation) => void;
  // 模糊过滤词（仅过滤「已加载」节点；命中字符高亮，命中目录自动展开）。空 = 不过滤。
  searchQuery?: string;
}

type MenuState = { x: number; y: number; entry: FileEntry | null };
type DialogState = { mode: 'newFile' | 'newDir' | 'rename'; dir: string; entry?: FileEntry };

// fuzzyIndices 子序列匹配：返回 query 各字符在 name 中命中的下标（用于高亮）；不匹配返回 null。
const fuzzyIndices = (name: string, query: string): number[] | null => {
  const n = name.toLowerCase();
  const idx: number[] = [];
  let qi = 0;
  for (let i = 0; i < n.length && qi < query.length; i++) {
    if (n[i] === query[qi]) {
      idx.push(i);
      qi += 1;
    }
  }
  return qi === query.length ? idx : null;
};

// 懒加载文件系统目录树：根目录直接子项常驻，展开子目录时按需拉取并缓存。
// 目录在前、文件在后（后端已排序）。点目录设为当前上下文 + 展开，点文件打开。
// 右键行/空白处弹上下文菜单：新建 / 重命名 / 复制副本 / 移到回收站 / 复制路径 / 在系统中显示。
export const FileSystemTree = ({
  rootPath,
  selectedPath,
  onSelectFile,
  onActivateDir,
  onAfterMutate,
  searchQuery = '',
}: FileSystemTreeProps) => {
  const theme = useTheme();
  const styles = fileSystemTreeStyles(theme);
  const t = useT();
  const vm = useFileSystemTree(rootPath, onAfterMutate);

  const [menu, setMenu] = useState<MenuState | null>(null);
  const [dialog, setDialog] = useState<DialogState | null>(null);

  // —— 模糊过滤（仅作用于已加载子树）——
  const q = searchQuery.trim().toLowerCase();
  const filtering = q !== '';
  const matchCache = new Map<string, boolean>();
  // 节点自身命中、或任一「已加载」后代命中 → 该节点应显示（保留通往命中项的路径）。
  const subtreeMatches = (entry: FileEntry): boolean => {
    const cached = matchCache.get(entry.path);
    if (cached !== undefined) return cached;
    let res = fuzzyIndices(entry.name, q) !== null;
    if (!res && entry.isDir) {
      const kids = vm.childrenByPath[entry.path];
      if (kids) res = kids.some(subtreeMatches);
    }
    matchCache.set(entry.path, res);
    return res;
  };
  // 渲染文件名：过滤态下高亮命中字符。
  const renderName = (name: string): ReactNode => {
    if (!filtering) return name;
    const idx = fuzzyIndices(name, q);
    if (!idx) return name;
    const hit = new Set(idx);
    return Array.from(name).map((ch, i) =>
      hit.has(i) ? (
        <Box component="span" key={i} sx={styles.matchHL}>
          {ch}
        </Box>
      ) : (
        ch
      ),
    );
  };

  const handleRow = (entry: FileEntry): void => {
    if (entry.isDir) {
      vm.toggle(entry.path);
      onActivateDir?.(entry.path);
    } else {
      onSelectFile(entry);
    }
  };

  const openRowMenu = (e: MouseEvent, entry: FileEntry): void => {
    e.preventDefault();
    e.stopPropagation();
    setMenu({ x: e.clientX, y: e.clientY, entry });
  };
  const openBlankMenu = (e: MouseEvent): void => {
    e.preventDefault();
    setMenu({ x: e.clientX, y: e.clientY, entry: null });
  };
  const closeMenu = (): void => setMenu(null);

  // 写操作统一异常收口：失败弹原生错误对话框。
  const runOp = async (fn: () => Promise<void>): Promise<void> => {
    try {
      await fn();
    } catch (err) {
      await NativeDialogs.error(
        t('files.dialog.errorTitle'),
        err instanceof Error ? err.message : String(err),
      );
    }
  };

  const onDuplicate = (entry: FileEntry): void => {
    closeMenu();
    void runOp(() => vm.duplicateEntry(entry));
  };
  const onTrash = async (entry: FileEntry): Promise<void> => {
    closeMenu();
    if (entry.isDir) {
      const ok = await NativeDialogs.confirm({
        title: t('files.menu.trashTitle'),
        message: t('files.menu.trashDirConfirm', { name: entry.name }),
        okLabel: t('files.menu.trashConfirmBtn'),
        cancelLabel: t('common.cancel'),
      });
      if (!ok) return;
    }
    void runOp(() => vm.trashEntry(entry));
  };
  const onCopyPath = (entry: FileEntry): void => {
    closeMenu();
    void navigator.clipboard?.writeText(entry.path).catch(() => {
      /* 复制失败静默：非关键路径 */
    });
  };
  const onReveal = (entry: FileEntry): void => {
    closeMenu();
    void FileService.revealInOS(entry.path).catch(() => {
      /* reveal 失败静默 */
    });
  };

  const onDialogConfirm = (value: string): void => {
    const d = dialog;
    setDialog(null);
    if (!d) return;
    if (d.mode === 'newFile') void runOp(() => vm.createFile(d.dir, value));
    else if (d.mode === 'newDir') void runOp(() => vm.createDir(d.dir, value));
    else if (d.mode === 'rename' && d.entry) void runOp(() => vm.renameEntry(d.entry!, value));
  };

  const renderNodes = (entries: FileEntry[], depth: number): ReactNode =>
    (filtering ? entries.filter(subtreeMatches) : entries).map((entry) => {
      const kids = vm.childrenByPath[entry.path];
      // 过滤态：有命中后代的目录强制展开（显示通往命中项的路径）。
      const kidsMatch = filtering && entry.isDir && !!kids && kids.some(subtreeMatches);
      const isExpanded = filtering ? kidsMatch : vm.expanded.has(entry.path);
      const isSelected = entry.path === selectedPath;
      return (
        <Box key={entry.path}>
          <Box
            sx={isSelected ? styles.rowActive : styles.row}
            style={{ paddingLeft: depth * 14 + 6 }}
            onClick={() => handleRow(entry)}
            onContextMenu={(e) => openRowMenu(e, entry)}
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
              {renderName(entry.name)}
            </Typography>
          </Box>
          {entry.isDir && isExpanded && (
            <>
              {!filtering && vm.loading.has(entry.path) && (
                <Box sx={styles.hint} style={{ paddingLeft: (depth + 1) * 14 + 6 }}>
                  {t('files.tree.loading')}
                </Box>
              )}
              {!filtering && vm.errored.has(entry.path) && (
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

  const menuItems = (): ReactNode => {
    if (!menu) return null;
    const item = (
      key: string,
      icon: ReactNode,
      label: string,
      onClick: () => void,
      danger = false,
    ): ReactNode => (
      <MenuItem key={key} onClick={onClick} sx={danger ? styles.menuItemDanger : styles.menuItem}>
        <ListItemIcon sx={danger ? styles.menuIconDanger : styles.menuIcon}>{icon}</ListItemIcon>
        <ListItemText slotProps={{ primary: { sx: styles.menuLabel } }}>{label}</ListItemText>
      </MenuItem>
    );

    // 空白处：仅新建 + 刷新（作用于根目录）。
    if (menu.entry === null) {
      return [
        item('nd', <CreateNewFolderRoundedIcon fontSize="small" />, t('files.menu.newDir'), () => {
          closeMenu();
          setDialog({ mode: 'newDir', dir: rootPath });
        }),
        item('nf', <NoteAddRoundedIcon fontSize="small" />, t('files.menu.newFile'), () => {
          closeMenu();
          setDialog({ mode: 'newFile', dir: rootPath });
        }),
        <Divider key="d" sx={styles.menuDivider} />,
        item('rf', <RefreshRoundedIcon fontSize="small" />, t('files.menu.refresh'), () => {
          closeMenu();
          vm.reload(rootPath);
        }),
      ];
    }

    const en = menu.entry;
    return [
      ...(en.isDir
        ? [
            item('nd', <CreateNewFolderRoundedIcon fontSize="small" />, t('files.menu.newDir'), () => {
              closeMenu();
              setDialog({ mode: 'newDir', dir: en.path });
            }),
            item('nf', <NoteAddRoundedIcon fontSize="small" />, t('files.menu.newFile'), () => {
              closeMenu();
              setDialog({ mode: 'newFile', dir: en.path });
            }),
            <Divider key="d0" sx={styles.menuDivider} />,
          ]
        : []),
      item('rn', <DriveFileRenameOutlineRoundedIcon fontSize="small" />, t('files.menu.rename'), () => {
        closeMenu();
        setDialog({ mode: 'rename', dir: '', entry: en });
      }),
      item('dup', <ContentCopyRoundedIcon fontSize="small" />, t('files.menu.duplicate'), () =>
        onDuplicate(en),
      ),
      item(
        'tr',
        <DeleteOutlineRoundedIcon fontSize="small" />,
        t('files.menu.trash'),
        () => void onTrash(en),
        true,
      ),
      <Divider key="d1" sx={styles.menuDivider} />,
      item('cp', <LinkRoundedIcon fontSize="small" />, t('files.menu.copyPath'), () => onCopyPath(en)),
      item('rv', <OpenInNewRoundedIcon fontSize="small" />, t('files.menu.reveal'), () => onReveal(en)),
    ];
  };

  return (
    <Box
      sx={styles.root}
      role="tree"
      aria-label={t('files.tree.ariaLabel')}
      onContextMenu={openBlankMenu}
    >
      {vm.rootLoading && <Box sx={styles.hint}>{t('files.tree.loading')}</Box>}
      {vm.rootErrored && <Box sx={styles.errorHint}>{t('files.tree.error')}</Box>}
      {!vm.rootLoading && !vm.rootErrored && vm.rootEntries.length === 0 && (
        <Box sx={styles.hint}>{t('files.tree.empty')}</Box>
      )}
      {filtering && vm.rootEntries.length > 0 && !vm.rootEntries.some(subtreeMatches) && (
        <Box sx={styles.hint}>{t('files.search.noMatch')}</Box>
      )}
      {renderNodes(vm.rootEntries, 0)}

      <Menu
        open={!!menu}
        onClose={closeMenu}
        anchorReference="anchorPosition"
        anchorPosition={menu ? { top: menu.y, left: menu.x } : undefined}
        slotProps={{ paper: { sx: styles.menuPaper } }}
      >
        {menuItems()}
      </Menu>

      <InputDialog
        open={!!dialog}
        title={dialog ? t(`files.dialog.${dialog.mode}.title`) : ''}
        label={dialog ? t(`files.dialog.${dialog.mode}.label`) : ''}
        confirmText={t('files.dialog.confirm')}
        cancelText={t('common.cancel')}
        initialValue={dialog?.mode === 'rename' ? (dialog.entry?.name ?? '') : ''}
        onConfirm={onDialogConfirm}
        onClose={() => setDialog(null)}
      />
    </Box>
  );
};
