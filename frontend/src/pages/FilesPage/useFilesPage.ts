import { useCallback, useEffect, useRef, useState } from 'react';
import { useT } from '@/i18n';
import { useCurrentDir } from '@/currentdir';
import { useRightPanel, type RightPanelId } from '@/rightpanel';
import { useTerminal } from '@/terminal';
import { useRouter } from '@/router';
import type { FileTreeMutation } from '@/components/FileSystemTree';
import {
  ClipboardService,
  FileService,
  GitService,
  LocalHistoryService,
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
  // 就地 diff（diffMode=true 时右栏渲染 DiffView）；diffSource 标明来源以区分工具栏按钮激活态。
  diffMode: boolean;
  diffSource: 'git' | 'clipboard' | null;
  gitDiff: DiffResult | null;
  diffLoading: boolean;
  diffError: 'notRepo' | 'generic' | 'clipboardEmpty' | null;
  toggleDiff: () => Promise<void>;
  // 与系统剪贴板文本比较（当前编辑器内容 vs 剪贴板）；空剪贴板友好提示。
  compareWithClipboard: () => Promise<void>;
  // 选中（驱动树高亮）
  selectedFile: string | null;
  // 编辑器跳转目标（内容搜索点击结果时定位行 + 高亮命中）；普通打开为 null。
  editorReveal: { line: number; term: string; nonce: number } | null;
  // 文件树内模糊过滤词（名称模式过滤已加载节点；内容模式作为全文搜索词）
  treeQuery: string;
  setTreeQuery: (q: string) => void;
  // 搜索模式：'name' 过滤已加载文件名 / 'content' 全文内容搜索（回车触发）
  searchMode: SearchMode;
  setSearchMode: (m: SearchMode) => void;
  contentResults: ContentMatch[];
  // 产出当前结果集的搜索词（结果列表命中高亮用）。
  contentTerm: string;
  contentLoading: boolean;
  contentSearched: boolean;
  runContentSearch: () => Promise<void>;
  openContentResult: (m: ContentMatch) => void;
  // 动作
  openFolder: () => Promise<void>;
  openRecent: (path: string) => void;
  selectFile: (entry: FileEntry) => Promise<void>;
  activateDir: (path: string) => void;
  // 在集成终端打开某目录：新建本地裸 shell 会话（cwd=该目录）并跳到终端页。
  openInTerminal: (path: string) => void;
  setEditValue: (v: string) => void;
  save: () => Promise<void>;
  setViewMode: (m: FilesViewMode) => void;
  reveal: () => void;
  // 右侧 Git / 本地历史抽屉开关（入口在本页头部工具栏，绑定当前目录）
  activePanel: RightPanelId | null;
  togglePanel: (id: RightPanelId) => void;
  // 文件树写操作通知：当前打开的文件被删除 / 改名时同步清空编辑器。
  onTreeMutate: (m: FileTreeMutation) => void;
  // Ctrl+P 快速打开浮层（仅文件页内触发，避让终端 Ctrl+P）。
  quickOpenOpen: boolean;
  closeQuickOpen: () => void;
  // 按绝对路径打开文件（QuickOpen 选中），可带 :line 跳转。
  openFileByPath: (path: string, line?: number) => void;
}

const dirOf = (p: string): string => p.replace(/[\\/][^\\/]*$/, '');
const baseOf = (p: string): string => p.replace(/\\/g, '/').split('/').pop() ?? p;
const isMd = (p: string): boolean => /\.(md|markdown)$/i.test(p);
const isImg = (p: string): boolean => /\.(png|jpe?g|gif|webp|bmp|svg|ico|avif)$/i.test(p);

export const useFilesPage = (): UseFilesPageResult => {
  const t = useT();
  const { rootDir, currentFile, setCurrentDir, setCurrentFile, openInBrowser } = useCurrentDir();
  const { active: activePanel, toggle: togglePanel } = useRightPanel();
  const { launchProject } = useTerminal();
  const { navigate, active } = useRouter();

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
  const [contentTerm, setContentTerm] = useState(''); // 产出当前结果集的搜索词（高亮用）
  const [editorReveal, setEditorReveal] = useState<{
    line: number;
    term: string;
    nonce: number;
  } | null>(null);
  const revealNonceRef = useRef(0);
  const [diffMode, setDiffMode] = useState(false);
  const [diffSource, setDiffSource] = useState<'git' | 'clipboard' | null>(null);
  const [gitDiff, setGitDiff] = useState<DiffResult | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [diffError, setDiffError] = useState<'notRepo' | 'generic' | 'clipboardEmpty' | null>(null);
  const [quickOpenOpen, setQuickOpenOpen] = useState(false);

  // Ctrl+P 全局监听用 ref 取最新的根目录 / 当前路由，避免重复绑定 / 闭包陈旧。
  const rootDirRef = useRef<string | null>(rootDir);
  rootDirRef.current = rootDir;
  const activeIdRef = useRef(active.id);
  activeIdRef.current = active.id;

  // 用 ref 追踪当前打开文件 / 编辑器内容，使依赖它们的回调保持稳定（不随每次输入重建）。
  const openFileRef = useRef<string | null>(null);
  useEffect(() => {
    openFileRef.current = openFilePath;
  }, [openFilePath]);
  const editValueRef = useRef(editValue);
  editValueRef.current = editValue;

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
    setDiffSource(null);
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
    async (entry: FileEntry, revealTarget?: { line: number; term: string }) => {
      if (entry.isDir) return;
      setLoadingFile(true);
      setFileError(null);
      setCurrentFile(entry.path);
      setCurrentDir(dirOf(entry.path));
      setImageDataUri(null);
      setDiffMode(false);
      setDiffSource(null);
      setGitDiff(null);
      setDiffError(null);
      setEditorReveal(null); // 默认清除跳转；命中结果在加载成功后再设
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
          // 文本加载成功且来自内容搜索结果：定位行 + 高亮命中。
          if (revealTarget && !fc.isBinary) {
            revealNonceRef.current += 1;
            setEditorReveal({
              line: revealTarget.line,
              term: revealTarget.term,
              nonce: revealNonceRef.current,
            });
          }
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

  // 按绝对路径打开文件（QuickOpen 选中）：合成 FileEntry 走 selectFile；带 line 时定位行。
  const openFileByPath = useCallback(
    (path: string, line?: number) => {
      const entry: FileEntry = { name: baseOf(path), path, isDir: false, size: 0, modTime: 0 };
      void selectFile(entry, line ? { line, term: '' } : undefined);
    },
    [selectFile],
  );

  const activateDir = useCallback(
    (path: string) => {
      setCurrentDir(path);
    },
    [setCurrentDir],
  );

  // Ctrl/Cmd+P 在文件页内打开快速检索浮层（仅当前路由为 files 且已打开根目录时；避让终端 Ctrl+P）。
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (activeIdRef.current !== 'files' || !rootDirRef.current) return;
      if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && (e.key === 'p' || e.key === 'P')) {
        e.preventDefault();
        setQuickOpenOpen(true);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const closeQuickOpen = useCallback(() => setQuickOpenOpen(false), []);

  // 在集成终端打开目录：复用 launchProject（cliTool='none' 裸 shell，projectPath=cwd）后跳终端页。
  // 非注册的临时目录也能开——后端建会话只用 cwd，不强制项目注册（参照 SshPage connect）。
  const openInTerminal = useCallback(
    (path: string) => {
      launchProject({
        projectId: `dir:${path}`,
        projectName: baseOf(path),
        projectPath: path,
        cliTool: 'none',
      });
      navigate('terminal');
    },
    [launchProject, navigate],
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
    setContentTerm(q); // 记下产出此结果集的词，供点击结果时高亮命中
    try {
      setContentResults(await FileService.searchContent(rootDir, q));
    } catch {
      setContentResults([]);
    } finally {
      setContentLoading(false);
    }
  }, [rootDir, treeQuery]);

  // 打开一条内容命中（复用 selectFile，构造最小 FileEntry）：带上行号 + 命中词定位高亮。
  const openContentResult = useCallback(
    (m: ContentMatch) => {
      void selectFile(
        { name: baseOf(m.path), path: m.path, isDir: false, size: 0, modTime: 0 },
        { line: m.line, term: contentTerm },
      );
    },
    [selectFile, contentTerm],
  );

  // 内容模式下清空搜索词即清空结果。
  useEffect(() => {
    if (searchMode === 'content' && treeQuery.trim() === '') {
      setContentResults([]);
      setContentSearched(false);
    }
  }, [searchMode, treeQuery]);

  // 切换「查看改动」：开启时拉取该文件 HEAD vs 工作树 diff（读盘内容，未保存的编辑不计入）。
  // 已在 Git diff 态再点 = 关闭；在剪贴板 diff 态点 = 切到 Git diff。
  const toggleDiff = useCallback(async () => {
    const open = openFileRef.current;
    if (!open) return;
    if (diffMode && diffSource === 'git') {
      setDiffMode(false);
      setDiffSource(null);
      return;
    }
    setDiffMode(true);
    setDiffSource('git');
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
  }, [diffMode, diffSource]);

  // 与系统剪贴板文本比较：编辑器当前内容(old) vs 剪贴板文本(new)；空剪贴板友好提示。
  // 剪贴板读取走 Wails runtime（ClipboardService），diff 走后端 ComputeDiff（DiffText）。
  const compareWithClipboard = useCallback(async () => {
    if (diffMode && diffSource === 'clipboard') {
      setDiffMode(false);
      setDiffSource(null);
      return;
    }
    setDiffMode(true);
    setDiffSource('clipboard');
    setDiffLoading(true);
    setDiffError(null);
    setGitDiff(null);
    try {
      const clip = await ClipboardService.readText();
      if (!clip.trim()) {
        setDiffError('clipboardEmpty');
        return;
      }
      setGitDiff(await LocalHistoryService.diffText(editValueRef.current, clip));
    } catch {
      setDiffError('generic');
    } finally {
      setDiffLoading(false);
    }
  }, [diffMode, diffSource]);

  // 文件树写操作回调：
  //   - 删除 / 改名当前打开的文件 → 清空编辑器与选中态（路径已不可用）。
  //   - 移动当前打开的文件 → 内容不变，仅跟随到新路径（保留未保存的编辑）。
  const onTreeMutate = useCallback(
    (m: FileTreeMutation) => {
      const open = openFileRef.current;
      if (!open) return;
      if (m.kind === 'move' && m.oldPath === open && m.newPath) {
        setOpenFilePath(m.newPath);
        setCurrentFile(m.newPath);
        setCurrentDir(dirOf(m.newPath));
        // 移动即退出 diff 视图：旧路径的 Git diff 已对不上新路径，按钮也不应残留激活。
        setDiffMode(false);
        setDiffSource(null);
        setGitDiff(null);
        return;
      }
      if ((m.kind === 'trash' || m.kind === 'rename') && m.oldPath === open) {
        setOpenFilePath(null);
        setEditValueState('');
        setOriginalValue('');
        setIsBinary(false);
        setImageDataUri(null);
        setDiffMode(false);
        setDiffSource(null);
        setGitDiff(null);
        setFileError(null);
        setCurrentFile(null);
      }
    },
    [setCurrentFile, setCurrentDir],
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
    diffSource,
    gitDiff,
    diffLoading,
    diffError,
    toggleDiff,
    compareWithClipboard,
    selectedFile: currentFile,
    editorReveal,
    treeQuery,
    setTreeQuery,
    searchMode,
    setSearchMode,
    contentResults,
    contentTerm,
    contentLoading,
    contentSearched,
    runContentSearch,
    openContentResult,
    openFolder,
    openRecent,
    selectFile,
    activateDir,
    openInTerminal,
    setEditValue,
    save,
    setViewMode,
    reveal,
    activePanel,
    togglePanel,
    onTreeMutate,
    quickOpenOpen,
    closeQuickOpen,
    openFileByPath,
  };
};

export const filesPathBase = baseOf;
