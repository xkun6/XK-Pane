import { useCallback, useEffect, useRef, useState } from 'react';
import { useT } from '@/i18n';
import { useCurrentDir } from '@/currentdir';
import { useRightPanel, type RightPanelId } from '@/rightpanel';
import type { FileTreeMutation } from '@/components/FileSystemTree';
import {
  FileService,
  GitService,
  NativeDialogs,
  type ContentMatch,
  type DiffResult,
  type FileEntry,
  type RecentDirectory,
} from '@/services';
import { isDesktop } from '@/services/transport';

export type FilesViewMode = 'source' | 'preview';
export type SearchMode = 'name' | 'content';

export interface UseFilesPageResult {
  isDesktop: boolean;
  rootDir: string | null;
  recent: RecentDirectory[];
  // 编辑器
  openFilePath: string | null;
  editValue: string;
  originalValue: string;
  dirty: boolean;
  saving: boolean;
  loadingFile: boolean;
  fileError: string | null;
  isBinary: boolean;
  isMarkdown: boolean;
  // 图片就地预览：非空时右栏渲染 <img>（data URI），不进文本编辑器。
  imageDataUri: string | null;
  viewMode: FilesViewMode;
  // Git diff 就地（HEAD vs 工作树）：diffMode=true 时右栏渲染 DiffView。
  diffMode: boolean;
  gitDiff: DiffResult | null;
  diffLoading: boolean;
  diffError: 'notRepo' | 'generic' | null;
  toggleDiff: () => Promise<void>;
  // 选中（驱动树高亮）
  selectedFile: string | null;
  // 文件树内模糊过滤词（名称模式过滤已加载节点；内容模式作为全文搜索词）
  treeQuery: string;
  setTreeQuery: (q: string) => void;
  // 搜索模式：'name' 过滤已加载文件名 / 'content' 全文内容搜索（回车触发）
  searchMode: SearchMode;
  setSearchMode: (m: SearchMode) => void;
  contentResults: ContentMatch[];
  contentLoading: boolean;
  contentSearched: boolean;
  runContentSearch: () => Promise<void>;
  openContentResult: (m: ContentMatch) => void;
  // 动作
  openFolder: () => Promise<void>;
  openRecent: (path: string) => void;
  selectFile: (entry: FileEntry) => Promise<void>;
  activateDir: (path: string) => void;
  setEditValue: (v: string) => void;
  save: () => Promise<void>;
  setViewMode: (m: FilesViewMode) => void;
  reveal: () => void;
  // 右侧 Git / 本地历史抽屉开关（入口在本页头部工具栏，绑定当前目录）
  activePanel: RightPanelId | null;
  togglePanel: (id: RightPanelId) => void;
  // 文件树写操作通知：当前打开的文件被删除 / 改名时同步清空编辑器。
  onTreeMutate: (m: FileTreeMutation) => void;
}

const dirOf = (p: string): string => p.replace(/[\\/][^\\/]*$/, '');
const baseOf = (p: string): string => p.replace(/\\/g, '/').split('/').pop() ?? p;
const isMd = (p: string): boolean => /\.(md|markdown)$/i.test(p);
const isImg = (p: string): boolean => /\.(png|jpe?g|gif|webp|bmp|svg|ico|avif)$/i.test(p);

export const useFilesPage = (): UseFilesPageResult => {
  const t = useT();
  const { rootDir, currentFile, setCurrentDir, setCurrentFile, openInBrowser } = useCurrentDir();
  const { active: activePanel, toggle: togglePanel } = useRightPanel();

  const [recent, setRecent] = useState<RecentDirectory[]>([]);
  const [openFilePath, setOpenFilePath] = useState<string | null>(null);
  const [editValue, setEditValueState] = useState('');
  const [originalValue, setOriginalValue] = useState('');
  const [saving, setSaving] = useState(false);
  const [loadingFile, setLoadingFile] = useState(false);
  const [fileError, setFileError] = useState<string | null>(null);
  const [isBinary, setIsBinary] = useState(false);
  const [imageDataUri, setImageDataUri] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<FilesViewMode>('source');
  const [treeQuery, setTreeQuery] = useState('');
  const [searchMode, setSearchModeState] = useState<SearchMode>('name');
  const [contentResults, setContentResults] = useState<ContentMatch[]>([]);
  const [contentLoading, setContentLoading] = useState(false);
  const [contentSearched, setContentSearched] = useState(false);
  const [diffMode, setDiffMode] = useState(false);
  const [gitDiff, setGitDiff] = useState<DiffResult | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [diffError, setDiffError] = useState<'notRepo' | 'generic' | null>(null);

  // 用 ref 追踪当前打开文件，使 onTreeMutate 回调保持稳定（不随每次 open 重建）。
  const openFileRef = useRef<string | null>(null);
  useEffect(() => {
    openFileRef.current = openFilePath;
  }, [openFilePath]);

  const refreshRecent = useCallback(() => {
    if (!isDesktop) return;
    void FileService.listRecent(20)
      .then(setRecent)
      .catch(() => setRecent([]));
  }, []);

  // 挂载拉取最近目录。
  useEffect(() => {
    refreshRecent();
  }, [refreshRecent]);

  // 切换根目录时清空编辑器（打开的文件可能不在新根下）。
  useEffect(() => {
    setOpenFilePath(null);
    setEditValueState('');
    setOriginalValue('');
    setIsBinary(false);
    setImageDataUri(null);
    setDiffMode(false);
    setGitDiff(null);
    setDiffError(null);
    setFileError(null);
    setTreeQuery('');
    setSearchModeState('name');
    setContentResults([]);
    setContentSearched(false);
  }, [rootDir]);

  const openFolder = useCallback(async () => {
    const dir = await NativeDialogs.openDirectory({ title: t('files.openFolderTitle') });
    if (!dir) return;
    openInBrowser(dir);
    void FileService.recordRecent(dir).then(refreshRecent).catch(refreshRecent);
  }, [t, openInBrowser, refreshRecent]);

  const openRecent = useCallback(
    (path: string) => {
      openInBrowser(path);
      void FileService.recordRecent(path).then(refreshRecent).catch(refreshRecent);
    },
    [openInBrowser, refreshRecent],
  );

  const selectFile = useCallback(
    async (entry: FileEntry) => {
      if (entry.isDir) return;
      setLoadingFile(true);
      setFileError(null);
      setCurrentFile(entry.path);
      setCurrentDir(dirOf(entry.path));
      setImageDataUri(null);
      setDiffMode(false);
      setGitDiff(null);
      setDiffError(null);
      try {
        if (isImg(entry.path)) {
          // 图片：读 data URI 就地预览，不进文本编辑器。
          const uri = await FileService.readFileBase64(entry.path);
          setOpenFilePath(entry.path);
          setIsBinary(true);
          setImageDataUri(uri);
          setEditValueState('');
          setOriginalValue('');
        } else {
          const fc = await FileService.readFile(entry.path);
          setOpenFilePath(entry.path);
          setIsBinary(fc.isBinary);
          setEditValueState(fc.content);
          setOriginalValue(fc.content);
        }
        setViewMode('source'); // 打开默认看源码（markdown 可在工具栏切预览）
      } catch (err) {
        setOpenFilePath(entry.path);
        setIsBinary(false);
        setImageDataUri(null);
        setEditValueState('');
        setOriginalValue('');
        setFileError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoadingFile(false);
      }
    },
    [setCurrentDir, setCurrentFile],
  );

  const activateDir = useCallback(
    (path: string) => {
      setCurrentDir(path);
    },
    [setCurrentDir],
  );

  const setEditValue = useCallback((v: string) => setEditValueState(v), []);

  const save = useCallback(async () => {
    if (!openFilePath || saving) return;
    setSaving(true);
    try {
      await FileService.writeFile(openFilePath, editValue);
      setOriginalValue(editValue);
    } catch (err) {
      await NativeDialogs.error(
        t('files.saveErrorTitle'),
        err instanceof Error ? err.message : String(err),
      );
    } finally {
      setSaving(false);
    }
  }, [openFilePath, editValue, saving, t]);

  const reveal = useCallback(() => {
    const target = currentFile ?? rootDir;
    if (!target) return;
    void FileService.revealInOS(target).catch(() => {
      /* reveal 失败静默：非关键路径 */
    });
  }, [currentFile, rootDir]);

  // 切换搜索模式（名称过滤 / 全文内容）：切换即清空上一次内容搜索结果。
  const setSearchMode = useCallback((m: SearchMode) => {
    setSearchModeState(m);
    setContentResults([]);
    setContentSearched(false);
  }, []);

  // 全文内容搜索（内容模式回车触发）：用当前搜索词在 rootDir 下 grep。
  const runContentSearch = useCallback(async () => {
    if (!rootDir) return;
    const q = treeQuery.trim();
    if (!q) {
      setContentResults([]);
      setContentSearched(false);
      return;
    }
    setContentLoading(true);
    setContentSearched(true);
    try {
      setContentResults(await FileService.searchContent(rootDir, q));
    } catch {
      setContentResults([]);
    } finally {
      setContentLoading(false);
    }
  }, [rootDir, treeQuery]);

  // 打开一条内容命中（复用 selectFile，构造最小 FileEntry）。
  const openContentResult = useCallback(
    (m: ContentMatch) => {
      void selectFile({ name: baseOf(m.path), path: m.path, isDir: false, size: 0, modTime: 0 });
    },
    [selectFile],
  );

  // 内容模式下清空搜索词即清空结果。
  useEffect(() => {
    if (searchMode === 'content' && treeQuery.trim() === '') {
      setContentResults([]);
      setContentSearched(false);
    }
  }, [searchMode, treeQuery]);

  // 切换「查看改动」：开启时拉取该文件 HEAD vs 工作树 diff（读盘内容，未保存的编辑不计入）。
  const toggleDiff = useCallback(async () => {
    const open = openFileRef.current;
    if (!open) return;
    if (diffMode) {
      setDiffMode(false);
      return;
    }
    setDiffMode(true);
    setDiffLoading(true);
    setDiffError(null);
    setGitDiff(null);
    try {
      const d = await GitService.diffFileAbs(open);
      setGitDiff(d);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setDiffError(/not a git repository/i.test(msg) ? 'notRepo' : 'generic');
    } finally {
      setDiffLoading(false);
    }
  }, [diffMode]);

  // 文件树写操作回调：当前打开的文件被删除 / 改名 → 清空编辑器与选中态（改名后路径已变）。
  const onTreeMutate = useCallback(
    (m: FileTreeMutation) => {
      const open = openFileRef.current;
      if (!open) return;
      if ((m.kind === 'trash' || m.kind === 'rename') && m.oldPath === open) {
        setOpenFilePath(null);
        setEditValueState('');
        setOriginalValue('');
        setIsBinary(false);
        setImageDataUri(null);
        setDiffMode(false);
        setGitDiff(null);
        setFileError(null);
        setCurrentFile(null);
      }
    },
    [setCurrentFile],
  );

  return {
    isDesktop,
    rootDir,
    recent,
    openFilePath,
    editValue,
    originalValue,
    dirty: editValue !== originalValue,
    saving,
    loadingFile,
    fileError,
    isBinary,
    isMarkdown: openFilePath ? isMd(openFilePath) : false,
    imageDataUri,
    viewMode,
    diffMode,
    gitDiff,
    diffLoading,
    diffError,
    toggleDiff,
    selectedFile: currentFile,
    treeQuery,
    setTreeQuery,
    searchMode,
    setSearchMode,
    contentResults,
    contentLoading,
    contentSearched,
    runContentSearch,
    openContentResult,
    openFolder,
    openRecent,
    selectFile,
    activateDir,
    setEditValue,
    save,
    setViewMode,
    reveal,
    activePanel,
    togglePanel,
    onTreeMutate,
  };
};

export const filesPathBase = baseOf;
