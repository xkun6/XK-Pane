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
import DifferenceRoundedIcon from '@mui/icons-material/DifferenceRounded';
import { useT } from '@/i18n';
import { CodeEditor, languageForPath } from '@/components/CodeEditor';
import { MarkdownView } from '@/components/MarkdownView';
import { DiffView } from '@/components/DiffView';
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
  // 「查看改动」仅对文本文件有意义（图片/二进制不 diff）。
  const canDiff = !vm.isBinary && !vm.imageDataUri;
  const diffLabels = {
    binary: t('files.diff.binary'),
    truncated: t('files.diff.truncated'),
    identical: t('files.diff.identical'),
    added: t('files.diff.added'),
    removed: t('files.diff.removed'),
  };

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
        {canDiff && (
          <Tooltip title={t('files.diff.view')} arrow>
            <span>
              <IconButton
                size="small"
                sx={vm.diffMode ? styles.toolBtnActive : styles.toolBtn}
                onClick={() => void vm.toggleDiff()}
                aria-label={t('files.diff.view')}
                aria-pressed={vm.diffMode}
              >
                <DifferenceRoundedIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
        )}
        <Tooltip title={t('files.editor.save')} arrow>
          <span>
            <IconButton
              size="small"
              sx={styles.toolBtn}
              disabled={!vm.dirty || vm.saving || vm.isBinary || vm.diffMode}
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
        ) : vm.diffMode ? (
          vm.diffLoading ? (
            <Box sx={styles.binaryHint}>{t('files.diff.loading')}</Box>
          ) : vm.diffError === 'notRepo' ? (
            <Box sx={styles.binaryHint}>{t('files.diff.notRepo')}</Box>
          ) : vm.diffError ? (
            <Box sx={styles.errorHint}>{t('files.diff.error')}</Box>
          ) : vm.gitDiff ? (
            <Box sx={styles.diffWrap}>
              <DiffView diff={vm.gitDiff} labels={diffLabels} />
            </Box>
          ) : null
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
            revealLine={vm.editorReveal?.line}
            revealTerm={vm.editorReveal?.term}
            revealNonce={vm.editorReveal?.nonce}
          />
        )}
      </Box>
    </Box>
  );
};
