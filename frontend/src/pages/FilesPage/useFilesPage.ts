import { useCallback, useEffect, useRef, useState } from 'react';
import { useT } from '@/i18n';
import { useCurrentDir } from '@/currentdir';
import { useRightPanel, type RightPanelId } from '@/rightpanel';
import type { FileTreeMutation } from '@/components/FileSystemTree';
import {
  FileService,
  NativeDialogs,
  type FileEntry,
  type RecentDirectory,
} from '@/services';
import { isDesktop } from '@/services/transport';

export type FilesViewMode = 'source' | 'preview';

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
  viewMode: FilesViewMode;
  // 选中（驱动树高亮）
  selectedFile: string | null;
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
  const [viewMode, setViewMode] = useState<FilesViewMode>('source');

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
    setFileError(null);
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
      try {
        const fc = await FileService.readFile(entry.path);
        setOpenFilePath(entry.path);
        setIsBinary(fc.isBinary);
        setEditValueState(fc.content);
        setOriginalValue(fc.content);
        setViewMode('source'); // 打开默认看源码（markdown 可在工具栏切预览）
      } catch (err) {
        setOpenFilePath(entry.path);
        setIsBinary(false);
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
    viewMode,
    selectedFile: currentFile,
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
