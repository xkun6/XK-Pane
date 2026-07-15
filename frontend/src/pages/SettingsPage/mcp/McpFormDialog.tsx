import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Switch,
  TextField,
  useTheme,
} from '@mui/material';
import DataObjectRoundedIcon from '@mui/icons-material/DataObjectRounded';
import { useT } from '@/i18n';
import { mcpStyles } from './Mcp.styles';
import type { JsonFieldError, McpDraft } from './useMcp';

export interface McpFormDialogProps {
  open: boolean;
  editingId: string | null;
  draft: McpDraft;
  nameValid: boolean;
  jsonError: JsonFieldError;
  onFieldChange: <K extends keyof McpDraft>(key: K, value: McpDraft[K]) => void;
  onFormat: () => void;
  onCancel: () => void;
  onSubmit: () => void;
}

// 新增/编辑 MCP server 的对话框（View，无业务）：名称 + serverJson 编辑器 + 格式化按钮 + 两个 CLI 启用开关。
export const McpFormDialog = ({
  open,
  editingId,
  draft,
  nameValid,
  jsonError,
  onFieldChange,
  onFormat,
  onCancel,
  onSubmit,
}: McpFormDialogProps) => {
  const theme = useTheme();
  const styles = mcpStyles(theme);
  const t = useT();

  const jsonErrorHelper = (): string | undefined => {
    if (jsonError === 'empty') return t('settings.mcp.form.jsonEmptyError');
    if (jsonError === 'invalid') return t('settings.mcp.form.jsonInvalidError');
    if (jsonError === 'multiple') return t('settings.mcp.form.jsonMultipleError');
    return undefined;
  };

  const nameTouched = draft.name.trim() !== '';
  const jsonTouched = draft.serverJson.trim() !== '';
  const showJsonError = jsonTouched && jsonError !== '';
  const canSave = nameValid && jsonError === '';

  return (
    <Dialog open={open} onClose={onCancel} maxWidth="sm" fullWidth>
      <DialogTitle>
        {editingId ? t('settings.mcp.form.editTitle') : t('settings.mcp.form.addTitle')}
      </DialogTitle>
      <DialogContent>
        <TextField
          label={t('settings.mcp.form.name')}
          placeholder={t('settings.mcp.form.namePlaceholder')}
          value={draft.name}
          onChange={(e) => onFieldChange('name', e.target.value)}
          fullWidth
          size="small"
          error={nameTouched && !nameValid}
          helperText={nameTouched && !nameValid ? t('settings.mcp.form.nameError') : undefined}
          sx={styles.formField}
        />
        <TextField
          label={t('settings.mcp.form.serverJson')}
          placeholder={t('settings.mcp.form.serverJsonPlaceholder')}
          value={draft.serverJson}
          onChange={(e) => onFieldChange('serverJson', e.target.value)}
          multiline
          minRows={10}
          fullWidth
          size="small"
          error={showJsonError}
          helperText={showJsonError ? jsonErrorHelper() : undefined}
          sx={showJsonError ? styles.editorError : styles.editor}
        />
        <Box sx={styles.formatRow}>
          <Button
            size="small"
            onClick={onFormat}
            startIcon={<DataObjectRoundedIcon fontSize="small" />}
            sx={{ textTransform: 'none' }}
          >
            {t('settings.mcp.form.format')}
          </Button>
        </Box>
        <Box sx={styles.toggleRow}>
          <FormControlLabel
            control={
              <Switch
                checked={draft.enabledClaude}
                onChange={(e) => onFieldChange('enabledClaude', e.target.checked)}
              />
            }
            label={t('settings.mcp.form.enableClaude')}
          />
          <FormControlLabel
            control={
              <Switch
                checked={draft.enabledCodex}
                onChange={(e) => onFieldChange('enabledCodex', e.target.checked)}
              />
            }
            label={t('settings.mcp.form.enableCodex')}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} sx={{ textTransform: 'none' }}>
          {t('settings.mcp.form.cancel')}
        </Button>
        <Button onClick={onSubmit} variant="contained" disabled={!canSave} sx={{ textTransform: 'none' }}>
          {t('settings.mcp.form.save')}
        </Button>
      </DialogActions>
    </Dialog>
  );
};
