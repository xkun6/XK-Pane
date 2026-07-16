import { Box, Button, useTheme } from '@mui/material';
import { Editor, type OnValidate } from '@monaco-editor/react';
import type { editor } from 'monaco-editor';
import { useCallback, useState } from 'react';
import DataObjectRoundedIcon from '@mui/icons-material/DataObjectRounded';
import { useT } from '@/i18n';
import { setupMonaco } from '@/monaco/monaco-setup';
import { jsonEditorFieldStyles } from './JsonEditorField.styles';

// 模块加载即配置 Monaco（worker + 本地 loader）；幂等。照抄 CodeEditor.tsx 的做法与理由：
// 首个编辑器挂载前需要一次性接管 loader，避免 @monaco-editor/react 默认从 CDN 拉 monaco。
setupMonaco();

export interface JsonEditorFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  height?: number;
  language?: 'json' | 'ini';
  error?: boolean;
  helperText?: string;
  placeholder?: string;
  onFormat?: () => void;
}

const EDITOR_OPTIONS = {
  minimap: { enabled: false },
  fontSize: 12.5,
  lineNumbers: 'on' as const,
  folding: true,
  wordWrap: 'on' as const,
  scrollBeyondLastLine: false,
  tabSize: 2,
  automaticLayout: true,
  renderLineHighlight: 'none' as const,
  overviewRulerLanes: 0,
  scrollbar: { verticalScrollbarSize: 8, horizontalScrollbarSize: 8 },
  stickyScroll: { enabled: false },
  contextmenu: false,
};

// 共享 Monaco JSON/INI 编辑器字段：语法高亮 + 实时行列级诊断（仅 json）+ 格式化。
// error/helperText 是外部（VM 纯函数）校验态；markers 是编辑器自身的实时反馈。
// helper 区显示优先级：外部 error 态 helperText > marker 行 > 普通 helperText。
export const JsonEditorField = ({
  label,
  value,
  onChange,
  height = 240,
  language = 'json',
  error,
  helperText,
  placeholder,
  onFormat,
}: JsonEditorFieldProps) => {
  const theme = useTheme();
  const styles = jsonEditorFieldStyles(theme);
  const t = useT();
  const monacoTheme = theme.palette.mode === 'dark' ? 'vs-dark' : 'vs';
  const [markerHint, setMarkerHint] = useState<string | undefined>(undefined);

  const handleValidate: OnValidate = useCallback(
    (markers: editor.IMarker[]) => {
      if (markers.length === 0) {
        setMarkerHint(undefined);
        return;
      }
      const worst = markers.reduce((a, b) => (b.severity > a.severity ? b : a));
      setMarkerHint(
        t('common.jsonEditor.markerAt', { line: worst.startLineNumber, column: worst.startColumn }) +
          worst.message,
      );
    },
    [t],
  );

  const handleFormat = useCallback(() => {
    if (onFormat) {
      onFormat();
      return;
    }
    try {
      const parsed: unknown = JSON.parse(value);
      onChange(JSON.stringify(parsed, null, 2));
    } catch {
      // 解析失败静默不动
    }
  }, [onFormat, onChange, value]);

  const showFormatButton = language === 'json' || Boolean(onFormat);
  const showPlaceholder = value.trim() === '' && Boolean(placeholder);

  let displayHelper: string | undefined;
  let displayIsError = false;
  if (error && helperText) {
    displayHelper = helperText;
    displayIsError = true;
  } else if (markerHint) {
    displayHelper = markerHint;
    displayIsError = true;
  } else if (helperText) {
    displayHelper = helperText;
  }

  return (
    <Box sx={styles.root}>
      <Box sx={styles.headerRow}>
        <Box component="span" sx={styles.label}>
          {label}
        </Box>
        {showFormatButton && (
          <Button
            size="small"
            onClick={handleFormat}
            startIcon={<DataObjectRoundedIcon fontSize="small" />}
            sx={styles.formatButton}
          >
            {t('common.jsonEditor.format')}
          </Button>
        )}
      </Box>
      <Box sx={error ? styles.editorContainerError : styles.editorContainer}>
        <Editor
          height={`${height}px`}
          language={language}
          value={value}
          theme={monacoTheme}
          onChange={(v) => onChange(v ?? '')}
          onValidate={handleValidate}
          options={EDITOR_OPTIONS}
        />
        {showPlaceholder && <Box sx={styles.placeholder}>{placeholder}</Box>}
      </Box>
      {displayHelper && <Box sx={displayIsError ? styles.helperTextError : styles.helperText}>{displayHelper}</Box>}
    </Box>
  );
};
