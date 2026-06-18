import {
  Box,
  Divider,
  InputBase,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Typography,
  useTheme,
  type SxProps,
  type Theme,
} from '@mui/material';
import ChevronRightRoundedIcon from '@mui/icons-material/ChevronRightRounded';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import CreateNewFolderRoundedIcon from '@mui/icons-material/CreateNewFolderRounded';
import NoteAddRoundedIcon from '@mui/icons-material/NoteAddRounded';
import DriveFileRenameOutlineRoundedIcon from '@mui/icons-material/DriveFileRenameOutlineRounded';
import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import ContentCutRoundedIcon from '@mui/icons-material/ContentCutRounded';
import ContentPasteRoundedIcon from '@mui/icons-material/ContentPasteRounded';
import FileCopyRoundedIcon from '@mui/icons-material/FileCopyRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import LinkRoundedIcon from '@mui/icons-material/LinkRounded';
import OpenInNewRoundedIcon from '@mui/icons-material/OpenInNewRounded';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import {
  useEffect,
  useRef,
  useState,
  type DragEvent,
  type MouseEvent,
  type ReactNode,
} from 'react';
import { useT } from '@/i18n';
import { FileService, NativeDialogs, type FileEntry } from '@/services';
import { FileTypeIcon } from '@/components/FileTypeIcon';
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
// 行内编辑态：新建（在 dir 下插幽灵输入行）或重命名（替换某行 name）。替代旧的模态对话框。
type EditingState =
  | { kind: 'newFile' | 'newDir'; dir: string }
  | { kind: 'rename'; entry: FileEntry };

// 模块级稳定组件：行内输入框（新建/重命名共用）。定义在组件外避免每次父渲染 remount 丢焦点。
// 挂载即聚焦并选区（重命名文件默认只选主干）；Enter 提交（校验通过时）/Esc 取消/blur 提交；
// 校验失败红框 + title 提示，不弹独立错误行（保持紧凑）。
interface InlineInputProps {
  initial: string;
  selectBasename: boolean;
  placeholder: string;
  validate: (name: string) => string | null; // 返回 i18n 后错误文案或 null
  onSubmit: (name: string) => void;
  onCancel: () => void;
  baseSx: SxProps<Theme> | undefined;
  errSx: SxProps<Theme> | undefined;
}
const InlineInput = ({
  initial,
  selectBasename,
  placeholder,
  validate,
  onSubmit,
  onCancel,
  baseSx,
  errSx,
}: InlineInputProps) => {
  const [val, setVal] = useState(initial);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const doneRef = useRef(false);
  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    el.focus();
    if (selectBasename) {
      const dot = initial.lastIndexOf('.');
      el.setSelectionRange(0, dot > 0 ? dot : initial.length);
    } else {
      el.select();
    }
    // 仅挂载时聚焦选区
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const err = val.trim() ? validate(val.trim()) : null;
  const finish = (commit: boolean): void => {
    if (doneRef.current) return;
    doneRef.current = true;
    if (commit && val.trim() && !err) onSubmit(val.trim());
    else onCancel();
  };
  return (
    <InputBase
      inputRef={inputRef}
      value={val}
      placeholder={placeholder}
      title={err ?? undefined}
      onChange={(e) => setVal(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault();
          if (val.trim() && !err) finish(true);
        } else if (e.key === 'Escape') {
          e.preventDefault();
          finish(false);
        }
      }}
      onBlur={() => finish(true)}
      onClick={(e) => e.stopPropagation()}
      spellCheck={false}
      sx={err ? ([baseSx, errSx] as SxProps<Theme>) : baseSx}
    />
  );
};

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
  const [editing, setEditing] = useState<EditingState | null>(null);
  // 拖拽移动：dragSrcRef 持当前被拖条目（dragover 阶段读不到 dataTransfer，故用 ref）；
  // dragOverPath 为正悬停的目录路径（驱动放置高亮）。
  const dragSrcRef = useRef<FileEntry | null>(null);
  const [dragOverPath, setDragOverPath] = useState<string | null>(null);

  const dirOf = (p: string): string => p.replace(/[\\/][^\\/]*$/, '');
  // 确保目录已展开（新建时让幽灵输入行可见）；已展开则不动。
  const ensureExpanded = (dir: string): void => {
    if (!vm.expanded.has(dir)) vm.toggle(dir);
  };
  // 构造名称校验闭包：空→不报错（由提交逻辑禁止）；非法字符/保留名/超长/同目录重名→返回 i18n 文案。
  // rename 传 selfName 以放行「纯大小写改名」（排除自身）。
  const makeValidate =
    (siblings: string[], selfName?: string) =>
    (name: string): string | null => {
      if (/[/\\\x00]/.test(name)) return t('files.inline.illegalChar');
      if (name === '.' || name === '..') return t('files.inline.dotName');
      if (name.length > 255) return t('files.inline.tooLong');
      const lower = name.toLowerCase();
      const self = selfName?.toLowerCase();
      if (siblings.some((s) => s.toLowerCase() === lower && s.toLowerCase() !== self)) {
        return t('files.inline.exists');
      }
      return null;
    };

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

  // —— 拖拽移动到文件夹 ——
  // 仅用于「是否拖入自身/子孙」的判定，归一仅作比较（不改实际移动用的原始路径，遵守 D7）。
  const normCmp = (p: string): string => p.replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
  const isInside = (srcPath: string, targetDir: string): boolean => {
    const a = normCmp(srcPath);
    const b = normCmp(targetDir);
    return b === a || b.startsWith(a + '/');
  };
  const performDrop = (targetDir: string): void => {
    const src = dragSrcRef.current;
    dragSrcRef.current = null;
    setDragOverPath(null);
    if (!src || isInside(src.path, targetDir)) return; // 拖入自身/子孙：忽略
    void runOp(() => vm.moveEntryInto(src, targetDir));
  };
  const onRowDragStart = (e: DragEvent, entry: FileEntry): void => {
    dragSrcRef.current = entry;
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', entry.path); // Firefox 需 setData 才能发起拖拽
  };
  const onRowDragEnd = (): void => {
    dragSrcRef.current = null;
    setDragOverPath(null);
  };
  const onDirDragOver = (e: DragEvent, dir: FileEntry): void => {
    if (!dragSrcRef.current) return;
    e.preventDefault();
    e.stopPropagation();
    const blocked = isInside(dragSrcRef.current.path, dir.path);
    e.dataTransfer.dropEffect = blocked ? 'none' : 'move';
    setDragOverPath(blocked ? null : dir.path);
  };
  const onDirDragLeave = (dir: FileEntry): void => {
    setDragOverPath((p) => (p === dir.path ? null : p));
  };
  const onDirDrop = (e: DragEvent, dir: FileEntry): void => {
    e.preventDefault();
    e.stopPropagation();
    performDrop(dir.path);
  };
  // 根容器：拖到空白/非目录处 = 移动到根目录（目录行的 drop 已 stopPropagation 不会到这）。
  const onRootDragOver = (e: DragEvent): void => {
    if (!dragSrcRef.current) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  };
  const onRootDrop = (e: DragEvent): void => {
    e.preventDefault();
    performDrop(rootPath);
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
  const onCopy = (entry: FileEntry): void => {
    closeMenu();
    vm.copyEntry(entry);
  };
  const onCut = (entry: FileEntry): void => {
    closeMenu();
    vm.cutEntry(entry);
  };
  const onPaste = (dir: string): void => {
    closeMenu();
    void runOp(() => vm.pasteInto(dir));
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
  // 相对 rootPath 的路径：归一仅用于判断前缀，切片用原始字符串以保留分隔符（守 D7，不规范化写入值）。
  const relativeToRoot = (abs: string): string => {
    const cleanRoot = rootPath.replace(/[\\/]+$/, '');
    if (cleanRoot && normCmp(abs).startsWith(normCmp(cleanRoot) + '/')) {
      return abs.slice(cleanRoot.length).replace(/^[\\/]+/, '');
    }
    return abs; // 不在根下：回退绝对路径
  };
  const onCopyRelPath = (entry: FileEntry): void => {
    closeMenu();
    void navigator.clipboard?.writeText(relativeToRoot(entry.path)).catch(() => {
      /* 复制失败静默 */
    });
  };
  const onReveal = (entry: FileEntry): void => {
    closeMenu();
    void FileService.revealInOS(entry.path).catch(() => {
      /* reveal 失败静默 */
    });
  };

  // 触发内联新建：先确保目标目录展开，再进入编辑态（幽灵行渲染在该目录子项顶部）。
  const startNew = (kind: 'newFile' | 'newDir', dir: string): void => {
    closeMenu();
    if (dir !== rootPath) ensureExpanded(dir);
    setEditing({ kind, dir });
  };
  const startRename = (entry: FileEntry): void => {
    closeMenu();
    setEditing({ kind: 'rename', entry });
  };

  // 幽灵新建输入行：editing 命中该 dir 时渲染，否则 null。
  const newRow = (dir: string, depth: number): ReactNode => {
    if (!editing || editing.kind === 'rename' || editing.dir !== dir) return null;
    const isDir = editing.kind === 'newDir';
    const siblings = (vm.childrenByPath[dir] ?? []).map((e) => e.name);
    return (
      <Box sx={styles.row} style={{ paddingLeft: depth * 14 + 6 }}>
        <Box sx={styles.chevronSpacer} aria-hidden />
        <Box sx={isDir ? styles.iconDir : styles.icon} aria-hidden>
          {isDir ? <FolderRoundedIcon fontSize="small" /> : <NoteAddRoundedIcon fontSize="small" />}
        </Box>
        <InlineInput
          initial=""
          selectBasename={false}
          placeholder={isDir ? t('files.dialog.newDir.label') : t('files.dialog.newFile.label')}
          validate={makeValidate(siblings)}
          onSubmit={(name) => {
            setEditing(null);
            void runOp(() => (isDir ? vm.createDir(dir, name) : vm.createFile(dir, name)));
          }}
          onCancel={() => setEditing(null)}
          baseSx={styles.inlineInput}
          errSx={styles.inlineInputError}
        />
      </Box>
    );
  };

  const renderNodes = (entries: FileEntry[], depth: number): ReactNode =>
    (filtering ? entries.filter(subtreeMatches) : entries).map((entry) => {
      const kids = vm.childrenByPath[entry.path];
      // 过滤态：有命中后代的目录强制展开（显示通往命中项的路径）。
      const kidsMatch = filtering && entry.isDir && !!kids && kids.some(subtreeMatches);
      const isExpanded = filtering ? kidsMatch : vm.expanded.has(entry.path);
      const isSelected = entry.path === selectedPath;
      const isDropTarget = entry.isDir && dragOverPath === entry.path;
      const isRenaming = editing?.kind === 'rename' && editing.entry.path === entry.path;
      return (
        <Box key={entry.path}>
          <Box
            sx={
              [
                isSelected ? styles.rowActive : styles.row,
                isDropTarget && styles.rowDropOverlay,
              ] as SxProps<Theme>
            }
            style={{ paddingLeft: depth * 14 + 6 }}
            onClick={() => {
              if (!isRenaming) handleRow(entry);
            }}
            onContextMenu={(e) => openRowMenu(e, entry)}
            draggable={!isRenaming}
            onDragStart={(e) => onRowDragStart(e, entry)}
            onDragEnd={onRowDragEnd}
            onDragOver={entry.isDir ? (e) => onDirDragOver(e, entry) : undefined}
            onDragLeave={entry.isDir ? () => onDirDragLeave(entry) : undefined}
            onDrop={entry.isDir ? (e) => onDirDrop(e, entry) : undefined}
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
                <FileTypeIcon name={entry.name} />
              )}
            </Box>
            {isRenaming ? (
              <InlineInput
                initial={entry.name}
                selectBasename={!entry.isDir}
                placeholder=""
                validate={makeValidate(
                  (vm.childrenByPath[dirOf(entry.path)] ?? []).map((e) => e.name),
                  entry.name,
                )}
                onSubmit={(name) => {
                  setEditing(null);
                  void runOp(() => vm.renameEntry(entry, name));
                }}
                onCancel={() => setEditing(null)}
                baseSx={styles.inlineInput}
                errSx={styles.inlineInputError}
              />
            ) : (
              <Typography component="span" sx={styles.name} title={entry.name}>
                {renderName(entry.name)}
              </Typography>
            )}
          </Box>
          {entry.isDir && isExpanded && (
            <>
              {newRow(entry.path, depth + 1)}
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

    // 空白处：新建 + 粘贴（剪贴板非空时，粘贴到根目录）+ 刷新。
    if (menu.entry === null) {
      return [
        item('nd', <CreateNewFolderRoundedIcon fontSize="small" />, t('files.menu.newDir'), () =>
          startNew('newDir', rootPath),
        ),
        item('nf', <NoteAddRoundedIcon fontSize="small" />, t('files.menu.newFile'), () =>
          startNew('newFile', rootPath),
        ),
        ...(vm.clipboard
          ? [
              item('pst', <ContentPasteRoundedIcon fontSize="small" />, t('files.menu.paste'), () =>
                onPaste(rootPath),
              ),
            ]
          : []),
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
            item('nd', <CreateNewFolderRoundedIcon fontSize="small" />, t('files.menu.newDir'), () =>
              startNew('newDir', en.path),
            ),
            item('nf', <NoteAddRoundedIcon fontSize="small" />, t('files.menu.newFile'), () =>
              startNew('newFile', en.path),
            ),
            ...(vm.clipboard
              ? [
                  item('pst', <ContentPasteRoundedIcon fontSize="small" />, t('files.menu.paste'), () =>
                    onPaste(en.path),
                  ),
                ]
              : []),
            <Divider key="d0" sx={styles.menuDivider} />,
          ]
        : []),
      item('rn', <DriveFileRenameOutlineRoundedIcon fontSize="small" />, t('files.menu.rename'), () =>
        startRename(en),
      ),
      item('cpy', <ContentCopyRoundedIcon fontSize="small" />, t('files.menu.copy'), () => onCopy(en)),
      item('cut', <ContentCutRoundedIcon fontSize="small" />, t('files.menu.cut'), () => onCut(en)),
      item('dup', <FileCopyRoundedIcon fontSize="small" />, t('files.menu.duplicate'), () =>
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
      item('crp', <LinkRoundedIcon fontSize="small" />, t('files.menu.copyRelPath'), () =>
        onCopyRelPath(en),
      ),
      item('rv', <OpenInNewRoundedIcon fontSize="small" />, t('files.menu.reveal'), () => onReveal(en)),
    ];
  };

  return (
    <Box
      sx={styles.root}
      role="tree"
      aria-label={t('files.tree.ariaLabel')}
      onContextMenu={openBlankMenu}
      onDragOver={onRootDragOver}
      onDrop={onRootDrop}
    >
      {vm.rootLoading && <Box sx={styles.hint}>{t('files.tree.loading')}</Box>}
      {vm.rootErrored && <Box sx={styles.errorHint}>{t('files.tree.error')}</Box>}
      {!vm.rootLoading && !vm.rootErrored && vm.rootEntries.length === 0 && (
        <Box sx={styles.hint}>{t('files.tree.empty')}</Box>
      )}
      {filtering && vm.rootEntries.length > 0 && !vm.rootEntries.some(subtreeMatches) && (
        <Box sx={styles.hint}>{t('files.search.noMatch')}</Box>
      )}
      {newRow(rootPath, 0)}
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
    </Box>
  );
};
