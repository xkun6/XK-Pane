import { Box, useTheme } from '@mui/material';
import { Editor, type OnMount } from '@monaco-editor/react';
import { useCallback, useEffect, useMemo, useRef } from 'react';
import { setupMonaco } from '@/monaco/monaco-setup';
import { languageForPath } from './languageForPath';

// 模块加载即配置 Monaco（worker + 本地 loader）；幂等。CodeEditor 走 lazy 路由，
// 故该副作用仅在首次进入文件页时触发。
setupMonaco();

type MountEditor = Parameters<OnMount>[0];
type MountMonaco = Parameters<OnMount>[1];

export interface CodeEditorProps {
  // 文件路径：用于按扩展名推断语言，并作为 key 强制换文件时重建编辑器（干净状态）。
  path: string;
  value: string;
  onChange?: (value: string) => void;
  readOnly?: boolean;
  // Ctrl/Cmd+S：在编辑器内注册保存命令（焦点在 Monaco 时也能触发）。
  onSave?: () => void;
  // 跳转定位：滚动到 revealLine 并选中该行内匹配 revealTerm 的文本（内容搜索点击结果用）。
  // revealNonce 每次点击递增，确保同行重复点击也重新跳转。
  revealLine?: number;
  revealTerm?: string;
  revealNonce?: number;
}

// Monaco 代码编辑器封装：语法高亮 + 编辑。主题跟随 foundation 明暗映射到 vs / vs-dark。
export const CodeEditor = ({
  path,
  value,
  onChange,
  readOnly,
  onSave,
  revealLine,
  revealTerm,
  revealNonce,
}: CodeEditorProps) => {
  const theme = useTheme();
  const monacoTheme = theme.palette.mode === 'dark' ? 'vs-dark' : 'vs';
  const language = useMemo(() => languageForPath(path), [path]);

  // onMount 只在挂载时跑一次：直接捕获 onSave 会钉死挂载那刻的闭包（saving 的是文件刚加载的
  // 原始内容，编辑后的值丢失）。用 ref 始终指向最新 onSave，命令回调取 ref.current 即拿到当前值。
  const onSaveRef = useRef(onSave);
  onSaveRef.current = onSave;

  const editorRef = useRef<MountEditor | null>(null);
  const monacoRef = useRef<MountMonaco | null>(null);

  // 跳到 revealLine 并选中行内匹配文本（无匹配则整行高亮）。
  const doReveal = useCallback(() => {
    const editor = editorRef.current;
    const monaco = monacoRef.current;
    if (!editor || !monaco || !revealLine || revealLine < 1) return;
    const model = editor.getModel();
    if (!model) return;
    const line = Math.min(revealLine, model.getLineCount());
    editor.revealLineInCenter(line);
    let range: InstanceType<MountMonaco['Range']> | null = null;
    if (revealTerm) {
      const idx = model.getLineContent(line).toLowerCase().indexOf(revealTerm.toLowerCase());
      if (idx >= 0) range = new monaco.Range(line, idx + 1, line, idx + 1 + revealTerm.length);
    }
    if (!range) range = new monaco.Range(line, 1, line, model.getLineMaxColumn(line));
    editor.setSelection(range);
    editor.focus();
  }, [revealLine, revealTerm]);

  const handleMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;
    monacoRef.current = monaco;
    // Ctrl/Cmd+S 保存（Monaco 自带 find/replace/undo/redo/注释/多光标/移动行等，无需另注册）。
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => onSaveRef.current?.());
    doReveal(); // 新文件挂载即跳转（若有目标）
  };

  // 同文件内换行（revealNonce 变化）时重新跳转。
  useEffect(() => {
    if (revealNonce !== undefined) doReveal();
  }, [revealNonce, doReveal]);

  return (
    <Box sx={{ flex: 1, minHeight: 0, minWidth: 0 }}>
      <Editor
        key={path}
        height="100%"
        language={language}
        value={value}
        theme={monacoTheme}
        onChange={(v) => onChange?.(v ?? '')}
        onMount={handleMount}
        options={{
          readOnly: readOnly ?? false,
          automaticLayout: true,
          minimap: { enabled: false },
          fontSize: 13,
          wordWrap: 'on',
          scrollBeyondLastLine: false,
          tabSize: 2,
          renderWhitespace: 'selection',
          smoothScrolling: true,
        }}
      />
    </Box>
  );
};
