import { useCallback, useEffect, useRef, useState } from 'react';
import { useT } from '@/i18n';
import { useCurrentDir } from '@/currentdir';
import { useRightPanel, type RightPanelId } from '@/rightpanel';
import { useTerminal } from '@/terminal';
import { useRouter } from '@/router';
import type { FileTreeMutation } from '@/components/FileSystemTree';
import { disposeAllFileModels, disposeFileModel } from '@/components/CodeEditor';
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

// 单个已打开文件的标签页状态（VS Code 式多标签：每个 tab 独立缓冲 + 独立 Monaco model）。
export interface FileTab {
  path: string;
  editValue: string;
  originalValue: string;
  isBinary: boolean;
  imageDataUri: string | null; // 图片 tab：data URI；非图片为 null
  fileError: string | null;
  viewMode: FilesViewMode; // markdown 源码/预览切换，每 tab 独立
  preview: boolean; // VS Code 预览 tab：斜体显示，会被下一次预览打开替换
}

export interface UseFilesPageResult {
  isDesktop: boolean;
  rootDir: string | null;
  recent: RecentDirectory[];
  // 编辑器（均从激活 tab 派生；无激活 tab 时给空默认值）
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
  // 文件树写操作通知：当前打开的文件被删除 / 改名 / 移动时同步所有匹配 tab。
  onTreeMutate: (m: FileTreeMutation) => void;
  // Ctrl+P 快速打开浮层（仅文件页内触发，避让终端 Ctrl+P）。
  quickOpenOpen: boolean;
  closeQuickOpen: () => void;
  // 按绝对路径打开文件（QuickOpen 选中），可带 :line 跳转。
  openFileByPath: (path: string, line?: number) => void;
  // 多文件标签页
  tabs: FileTab[];
  activateTab: (path: string) => void;
  closeTab: (path: string) => Promise<void>;
  pinTab: (path: string) => void;
  closeOtherTabs: (path: string) => Promise<void>;
  closeTabsToRight: (path: string) => Promise<void>;
  closeAllTabs: () => Promise<void>;
  reorderTabs: (fromPath: string, toPath: string) => void;
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
  const [tabs, setTabs] = useState<FileTab[]>([]);
  const [activePath, setActivePath] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [loadingFile, setLoadingFile] = useState(false);
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

  // 用 ref 追踪 tabs / activePath，使依赖它们的回调保持稳定（不随每次输入重建）。
  const tabsRef = useRef<FileTab[]>(tabs);
  const activePathRef = useRef<string | null>(null);
  // 待 dispose 的 Monaco model 路径队列：只在此处 push，统一由下方 effect 消费。
  const pendingDisposeRef = useRef<string[]>([]);

  const activeTab = tabs.find((tb) => tb.path === activePath) ?? null;

  // 当前编辑器内容的直读 ref（渲染期同步赋值，无一帧延迟），供 compareWithClipboard 等异步回调取用。
  const editValueRef = useRef('');
  editValueRef.current = activeTab?.editValue ?? '';

  useEffect(() => {
    activePathRef.current = activePath;
  }, [activePath]);

  // tabs 提交后：同步 ref + 冲洗待 dispose 队列（此时子组件 CodeEditor 已先一步把 Monaco 编辑器
  // 切到新 model，dispose 旧 model 不会破坏正在显示的内容）。
  useEffect(() => {
    tabsRef.current = tabs;
    if (pendingDisposeRef.current.length > 0) {
      for (const p of pendingDisposeRef.current) disposeFileModel(p);
      pendingDisposeRef.current = [];
    }
  }, [tabs]);

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

  // 切换根目录时关闭全部 tab（打开的文件可能不在新根下）并释放所有本页 model。
  useEffect(() => {
    setTabs([]);
    setActivePath(null);
    disposeAllFileModels();
    setDiffMode(false);
    setDiffSource(null);
    setGitDiff(null);
    setDiffError(null);
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

  // 打开/激活一个跳转目标后按需设置 editorReveal（无目标或目标 tab 非文本则清除）。
  const revealForTab = useCallback((tab: FileTab, revealTarget?: { line: number; term: string }) => {
    if (revealTarget && !tab.isBinary && !tab.imageDataUri) {
      revealNonceRef.current += 1;
      setEditorReveal({ line: revealTarget.line, term: revealTarget.term, nonce: revealNonceRef.current });
    } else {
      setEditorReveal(null);
    }
  }, []);

  // 打开路径的核心实现：selectFile / openFileByPath / openContentResult 都走它。
  //   - 已打开：只激活（不重新读盘，保留未保存的编辑）。
  //   - 未打开：读盘构造新 tab，按 VS Code 预览语义插入（至多一个 preview tab）。
  const openPath = useCallback(
    async (path: string, revealTarget?: { line: number; term: string }) => {
      setCurrentFile(path);
      setCurrentDir(dirOf(path));
      setDiffMode(false);
      setDiffSource(null);
      setGitDiff(null);
      setDiffError(null);

      const existing = tabsRef.current.find((tb) => tb.path === path);
      if (existing) {
        setActivePath(path);
        revealForTab(existing, revealTarget);
        return;
      }

      setLoadingFile(true);
      let newTab: FileTab;
      try {
        if (isImg(path)) {
          // 图片：读 data URI 就地预览，不进文本编辑器。
          const uri = await FileService.readFileBase64(path);
          newTab = {
            path,
            editValue: '',
            originalValue: '',
            isBinary: true,
            imageDataUri: uri,
            fileError: null,
            viewMode: 'source',
            preview: true,
          };
        } else {
          const fc = await FileService.readFile(path);
          newTab = {
            path,
            editValue: fc.content,
            originalValue: fc.content,
            isBinary: fc.isBinary,
            imageDataUri: null,
            fileError: null,
            viewMode: 'source',
            preview: true,
          };
        }
      } catch (err) {
        newTab = {
          path,
          editValue: '',
          originalValue: '',
          isBinary: false,
          imageDataUri: null,
          fileError: err instanceof Error ? err.message : String(err),
          viewMode: 'source',
          preview: true,
        };
      } finally {
        setLoadingFile(false);
      }

      // 插入策略（VS Code 预览语义）：已有 preview tab 且不脏 → 原位替换；脏（防御性）→ 固定后追加；
      // 否则直接追加到末尾。
      const prevTabs = tabsRef.current;
      const previewIdx = prevTabs.findIndex((tb) => tb.preview);
      const prevPreviewTab = previewIdx >= 0 ? prevTabs[previewIdx] : undefined;
      let nextTabs: FileTab[];
      if (prevPreviewTab) {
        if (prevPreviewTab.editValue === prevPreviewTab.originalValue) {
          pendingDisposeRef.current.push(prevPreviewTab.path);
          nextTabs = [...prevTabs];
          nextTabs[previewIdx] = newTab;
        } else {
          nextTabs = [...prevTabs];
          nextTabs[previewIdx] = { ...prevPreviewTab, preview: false };
          nextTabs.push(newTab);
        }
      } else {
        nextTabs = [...prevTabs, newTab];
      }
      setTabs(nextTabs);
      setActivePath(path);
      revealForTab(newTab, revealTarget);
    },
    [setCurrentDir, setCurrentFile, revealForTab],
  );

  const selectFile = useCallback(
    async (entry: FileEntry, revealTarget?: { line: number; term: string }) => {
      if (entry.isDir) return;
      await openPath(entry.path, revealTarget);
    },
    [openPath],
  );

  // 按绝对路径打开文件（QuickOpen 选中）；带 line 时定位行。
  const openFileByPath = useCallback(
    (path: string, line?: number) => {
      void openPath(path, line ? { line, term: '' } : undefined);
    },
    [openPath],
  );

  const activateDir = useCallback(
    (path: string) => {
      setCurrentDir(path);
    },
    [setCurrentDir],
  );

  const activateTab = useCallback(
    (path: string) => {
      setActivePath(path);
      setCurrentFile(path);
      setCurrentDir(dirOf(path));
      setDiffMode(false);
      setDiffSource(null);
      setGitDiff(null);
      setDiffError(null);
      setEditorReveal(null);
    },
    [setCurrentFile, setCurrentDir],
  );

  const pinTab = useCallback((path: string) => {
    setTabs((prev) => prev.map((tb) => (tb.path === path ? { ...tb, preview: false } : tb)));
  }, []);

  const closeTab = useCallback(
    async (path: string) => {
      const tab = tabsRef.current.find((tb) => tb.path === path);
      if (!tab) return;
      if (tab.editValue !== tab.originalValue) {
        const ok = await NativeDialogs.confirm({
          title: t('files.tabs.closeDirtyTitle'),
          message: t('files.tabs.closeDirtyMessage', { name: baseOf(path) }),
          okLabel: t('files.tabs.discard'),
          cancelLabel: t('files.tabs.cancel'),
        });
        if (!ok) return;
      }
      const prev = tabsRef.current;
      const idx = prev.findIndex((tb) => tb.path === path);
      if (idx < 0) return;
      pendingDisposeRef.current.push(path);
      const next = prev.filter((tb) => tb.path !== path);
      setTabs(next);
      if (activePathRef.current === path) {
        const neighbor = next[idx] ?? next[idx - 1] ?? null;
        if (neighbor) {
          setActivePath(neighbor.path);
          setCurrentFile(neighbor.path);
          setCurrentDir(dirOf(neighbor.path));
        } else {
          setActivePath(null);
          setCurrentFile(null);
        }
      }
    },
    [t, setCurrentFile, setCurrentDir],
  );

  // 批量关闭 tabs 的公共逻辑：脏 tab 数 > 0 时弹一次聚合确认（取消则整体不关）；
  // 确认通过后统一 dispose model + 从 tabs 移除。返回关闭后的最新 tabs；用户取消返回 null。
  const closeMany = useCallback(
    async (victims: FileTab[]): Promise<FileTab[] | null> => {
      if (victims.length === 0) return tabsRef.current;
      const dirtyCount = victims.filter((tb) => tb.editValue !== tb.originalValue).length;
      if (dirtyCount > 0) {
        const ok = await NativeDialogs.confirm({
          title: t('files.tabs.closeDirtyTitle'),
          message: t('files.tabs.closeManyDirtyMessage', { n: String(dirtyCount) }),
          okLabel: t('files.tabs.discard'),
          cancelLabel: t('files.tabs.cancel'),
        });
        if (!ok) return null;
      }
      // 确认等待期间 tabs 可能已变化，用最新 tabsRef 重新过滤受害者路径集合。
      const victimPaths = new Set(victims.map((tb) => tb.path));
      const latest = tabsRef.current;
      for (const tb of latest) {
        if (victimPaths.has(tb.path)) pendingDisposeRef.current.push(tb.path);
      }
      const next = latest.filter((tb) => !victimPaths.has(tb.path));
      setTabs(next);
      return next;
    },
    [t],
  );

  // 关闭除 path 外的全部 tab；若当前激活的不是 path，关闭后激活它。
  const closeOtherTabs = useCallback(
    async (path: string) => {
      const victims = tabsRef.current.filter((tb) => tb.path !== path);
      const next = await closeMany(victims);
      if (next === null) return;
      if (activePathRef.current !== path) activateTab(path);
    },
    [closeMany, activateTab],
  );

  // 关闭 path 右侧（含之后）全部 tab；若当前激活的 tab 被关闭，改激活 path。
  const closeTabsToRight = useCallback(
    async (path: string) => {
      const all = tabsRef.current;
      const idx = all.findIndex((tb) => tb.path === path);
      if (idx < 0) return;
      const victims = all.slice(idx + 1);
      const wasActiveClosed = victims.some((tb) => tb.path === activePathRef.current);
      const next = await closeMany(victims);
      if (next === null) return;
      if (wasActiveClosed) activateTab(path);
    },
    [closeMany, activateTab],
  );

  // 关闭全部 tab：清空激活态 + 当前文件，并退出 diff 视图。
  const closeAllTabs = useCallback(async () => {
    const next = await closeMany(tabsRef.current);
    if (next === null) return;
    setActivePath(null);
    setCurrentFile(null);
    setDiffMode(false);
    setDiffSource(null);
    setGitDiff(null);
    setDiffError(null);
  }, [closeMany, setCurrentFile]);

  // 拖拽排序：把 fromPath 从原位置移到 toPath 所在位置（纯数组重排，不碰 Monaco model）。
  const reorderTabs = useCallback((fromPath: string, toPath: string) => {
    setTabs((prev) => {
      const fromIdx = prev.findIndex((tb) => tb.path === fromPath);
      const toIdx = prev.findIndex((tb) => tb.path === toPath);
      if (fromIdx < 0 || toIdx < 0 || fromIdx === toIdx) return prev;
      const next = [...prev];
      const [moved] = next.splice(fromIdx, 1);
      if (!moved) return prev;
      next.splice(toIdx, 0, moved);
      return next;
    });
  }, []);

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

  // Ctrl+Tab / Ctrl+Shift+Tab 循环切换标签、Ctrl+W 关闭当前标签。capture 阶段注册，
  // 确保焦点落在 Monaco 编辑器内时依然能拦到；仅当前路由为 files 且存在 tab 时生效。
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (activeIdRef.current !== 'files' || tabsRef.current.length === 0) return;
      if (e.ctrlKey && e.key === 'Tab') {
        e.preventDefault();
        e.stopPropagation();
        const list = tabsRef.current;
        const curIdx = list.findIndex((tb) => tb.path === activePathRef.current);
        const base = curIdx >= 0 ? curIdx : 0;
        const delta = e.shiftKey ? -1 : 1;
        const nextTab = list[(base + delta + list.length) % list.length];
        if (nextTab) activateTab(nextTab.path);
        return;
      }
      if (e.ctrlKey && !e.shiftKey && !e.altKey && (e.key === 'w' || e.key === 'W')) {
        e.preventDefault();
        e.stopPropagation();
        const path = activePathRef.current;
        if (path) void closeTab(path);
      }
    };
    window.addEventListener('keydown', onKeyDown, true);
    return () => window.removeEventListener('keydown', onKeyDown, true);
  }, [activateTab, closeTab]);

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

  // 写入激活 tab 的 editValue；若该 tab 是 preview，同时置 preview=false（编辑即固定）。
  // 用函数式 setTabs + activePath 的 ref，保持回调引用稳定（不随输入重建，避免打字卡顿）。
  const setEditValue = useCallback((v: string) => {
    const path = activePathRef.current;
    if (!path) return;
    setTabs((prev) => prev.map((tb) => (tb.path === path ? { ...tb, editValue: v, preview: false } : tb)));
  }, []);

  const save = useCallback(async () => {
    const path = activePathRef.current;
    const tab = path ? tabsRef.current.find((tb) => tb.path === path) : undefined;
    if (!path || !tab || saving) return;
    const snapshot = tab.editValue; // 固定本次保存写盘的内容，避免 await 期间继续输入导致的错配
    setSaving(true);
    try {
      await FileService.writeFile(path, snapshot);
      setTabs((prev) => prev.map((tb) => (tb.path === path ? { ...tb, originalValue: snapshot } : tb)));
    } catch (err) {
      await NativeDialogs.error(
        t('files.saveErrorTitle'),
        err instanceof Error ? err.message : String(err),
      );
    } finally {
      setSaving(false);
    }
  }, [saving, t]);

  const setViewMode = useCallback((m: FilesViewMode) => {
    const path = activePathRef.current;
    if (!path) return;
    setTabs((prev) => prev.map((tb) => (tb.path === path ? { ...tb, viewMode: m } : tb)));
  }, []);

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

  // 打开一条内容命中：带上行号 + 命中词定位高亮。
  const openContentResult = useCallback(
    (m: ContentMatch) => {
      void openPath(m.path, { line: m.line, term: contentTerm });
    },
    [openPath, contentTerm],
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
    const open = activePathRef.current;
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

  // 文件树写操作回调（对所有匹配 tab 生效，不只当前激活的）：
  //   - 删除 / 改名匹配的 tab → 强制关闭（文件已没了，不弹脏确认）。
  //   - 移动匹配的 tab → 内容不变，仅跟随到新路径（保留未保存的编辑）。
  const onTreeMutate = useCallback(
    (m: FileTreeMutation) => {
      if (m.kind === 'move' && m.oldPath && m.newPath) {
        const oldPath = m.oldPath;
        const newPath = m.newPath;
        const prev = tabsRef.current;
        if (!prev.some((tb) => tb.path === oldPath)) return;
        pendingDisposeRef.current.push(oldPath);
        setTabs(prev.map((tb) => (tb.path === oldPath ? { ...tb, path: newPath } : tb)));
        if (activePathRef.current === oldPath) {
          setActivePath(newPath);
          setCurrentFile(newPath);
          setCurrentDir(dirOf(newPath));
          // 移动即退出 diff 视图：旧路径的 Git diff 已对不上新路径，按钮也不应残留激活。
          setDiffMode(false);
          setDiffSource(null);
          setGitDiff(null);
        }
        return;
      }
      if ((m.kind === 'trash' || m.kind === 'rename') && m.oldPath) {
        const oldPath = m.oldPath;
        const prev = tabsRef.current;
        const idx = prev.findIndex((tb) => tb.path === oldPath);
        if (idx < 0) return;
        pendingDisposeRef.current.push(oldPath);
        const next = prev.filter((tb) => tb.path !== oldPath);
        setTabs(next);
        if (activePathRef.current === oldPath) {
          const neighbor = next[idx] ?? next[idx - 1] ?? null;
          if (neighbor) {
            setActivePath(neighbor.path);
            setCurrentFile(neighbor.path);
            setCurrentDir(dirOf(neighbor.path));
          } else {
            setActivePath(null);
            setCurrentFile(null);
          }
          setDiffMode(false);
          setDiffSource(null);
          setGitDiff(null);
        }
      }
    },
    [setCurrentFile, setCurrentDir],
  );

  return {
    isDesktop,
    rootDir,
    recent,
    openFilePath: activePath,
    editValue: activeTab?.editValue ?? '',
    originalValue: activeTab?.originalValue ?? '',
    dirty: activeTab ? activeTab.editValue !== activeTab.originalValue : false,
    saving,
    loadingFile,
    fileError: activeTab?.fileError ?? null,
    isBinary: activeTab?.isBinary ?? false,
    isMarkdown: activePath ? isMd(activePath) : false,
    imageDataUri: activeTab?.imageDataUri ?? null,
    viewMode: activeTab?.viewMode ?? 'source',
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
    tabs,
    activateTab,
    closeTab,
    pinTab,
    closeOtherTabs,
    closeTabsToRight,
    closeAllTabs,
    reorderTabs,
  };
};

export const filesPathBase = baseOf;
