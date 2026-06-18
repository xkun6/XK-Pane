import {
  Box,
  IconButton,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import { useT } from '@/i18n';
import { CodeEditor, languageForPath } from '@/components/CodeEditor';
import { MarkdownView } from '@/components/MarkdownView';
import { filesPageStyles } from './FilesPage.styles';
import { filesPathBase, type FilesViewMode, type UseFilesPageResult } from './useFilesPage';

// 右栏编辑器面板：工具栏（文件名 + 脏标记 + 语言 + markdown 源/预览切换 + 保存）+ 编辑器主体。
export const FileEditorPane = ({ vm }: { vm: UseFilesPageResult }) => {
  const theme = useTheme();
  const styles = filesPageStyles(theme);
  const t = useT();

  if (!vm.openFilePath) {
    return <Box sx={styles.editorPlaceholder}>{t('files.editor.selectFile')}</Box>;
  }

  const lang = languageForPath(vm.openFilePath);

  return (
    <Box sx={styles.right}>
      <Box sx={styles.toolbar}>
        <Typography sx={styles.fileName} title={vm.openFilePath}>
          {filesPathBase(vm.openFilePath)}
        </Typography>
        {vm.dirty && <Box sx={styles.dirtyDot} aria-label={t('files.editor.dirty')} />}
        {!vm.imageDataUri && (
          <Box component="span" sx={styles.langChip}>
            {lang}
          </Box>
        )}
        <Box sx={styles.spacer} />
        {vm.isMarkdown && !vm.isBinary && (
          <ToggleButtonGroup
            size="small"
            exclusive
            value={vm.viewMode}
            onChange={(_, v) => {
              if (v) vm.setViewMode(v as FilesViewMode);
            }}
            sx={styles.toggleGroup}
          >
            <ToggleButton value="source">{t('files.editor.source')}</ToggleButton>
            <ToggleButton value="preview">{t('files.editor.preview')}</ToggleButton>
          </ToggleButtonGroup>
        )}
        <Tooltip title={t('files.editor.save')} arrow>
          <span>
            <IconButton
              size="small"
              sx={styles.toolBtn}
              disabled={!vm.dirty || vm.saving || vm.isBinary}
              onClick={() => void vm.save()}
              aria-label={t('files.editor.save')}
            >
              <SaveRoundedIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Box>
      <Box sx={styles.editorBody}>
        {vm.fileError ? (
          <Box sx={styles.errorHint}>{t('files.editor.loadError')}</Box>
        ) : vm.imageDataUri ? (
          <Box sx={styles.imageWrap}>
            <Box
              component="img"
              src={vm.imageDataUri}
              alt={filesPathBase(vm.openFilePath)}
              sx={styles.image}
            />
          </Box>
        ) : vm.isBinary ? (
          <Box sx={styles.binaryHint}>{t('files.editor.binary')}</Box>
        ) : vm.isMarkdown && vm.viewMode === 'preview' ? (
          <MarkdownView source={vm.editValue} />
        ) : (
          <CodeEditor
            path={vm.openFilePath}
            value={vm.editValue}
            onChange={vm.setEditValue}
            onSave={() => void vm.save()}
          />
        )}
      </Box>
    </Box>
  );
};
