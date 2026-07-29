import { useState } from 'react';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  TextField,
  Typography,
  useTheme,
} from '@mui/material';
import { useT } from '@/i18n';
import { JsonEditorField } from '@/components/JsonEditorField';
import type { AgentAppType } from '@/services';
import type { JsonFieldError } from './agentProviderJson';
import { globalProviderPanelStyles } from './GlobalProviderPanel.styles';
import type { ClaudeTemplateId, GlobalProviderDraft } from './useGlobalProviderPanel';

const CLAUDE_TEMPLATE_IDS: ClaudeTemplateId[] = ['blank', 'apiKey', 'thirdParty'];

export interface GlobalProviderFormDialogProps {
  appType: AgentAppType;
  open: boolean;
  editingId: string | null;
  editingIsCurrent: boolean;
  draft: GlobalProviderDraft;
  claudeJsonError: JsonFieldError;
  codexAuthJsonError: JsonFieldError;
  onFieldChange: <K extends keyof GlobalProviderDraft>(key: K, value: GlobalProviderDraft[K]) => void;
  onApplyTemplate: (template: ClaudeTemplateId) => void;
  onCancel: () => void;
  onSubmit: () => void;
}

// 新增/编辑 Provider 的对话框（View，无业务）：公共字段 name/notes + 按 appType 分叉的配置
// 编辑器（claude：单块 settings.json + 模板下拉；codex：auth.json + config.toml 两块）。
export const GlobalProviderFormDialog = ({
  appType,
  open,
  editingId,
  editingIsCurrent,
  draft,
  claudeJsonError,
  codexAuthJsonError,
  onFieldChange,
  onApplyTemplate,
  onCancel,
  onSubmit,
}: GlobalProviderFormDialogProps) => {
  const theme = useTheme();
  const styles = globalProviderPanelStyles(theme);
  const t = useT();
  // 模板下拉本身不持久化选择态——每次选中即触发一次性填充，随后交回 draft 编辑，
  // 纯 UI 展示状态，不算业务态。
  const [templateSelectValue, setTemplateSelectValue] = useState('');

  const appLabel = t(`settings.provider.tabs.${appType}`);

  const jsonErrorHelper = (code: JsonFieldError): string | undefined => {
    if (code === 'empty') return t('settings.provider.global.form.jsonEmptyError');
    if (code === 'invalid') return t('settings.provider.global.form.jsonInvalidError');
    return undefined;
  };

  const canSave =
    draft.name.trim() !== '' &&
    (appType === 'claude' ? claudeJsonError === '' : codexAuthJsonError === '');

  return (
    <Dialog open={open} onClose={onCancel} maxWidth="md" fullWidth>
      <DialogTitle>
        {editingId
          ? t('settings.provider.global.form.editTitle', { app: appLabel })
          : t('settings.provider.global.form.addTitle', { app: appLabel })}
      </DialogTitle>
      <DialogContent>
        <TextField
          label={t('settings.provider.global.form.name')}
          placeholder={t('settings.provider.global.form.namePlaceholder')}
          value={draft.name}
          onChange={(e) => onFieldChange('name', e.target.value)}
          fullWidth
          size="small"
          sx={styles.formField}
        />
        <TextField
          label={t('settings.provider.global.form.notes')}
          placeholder={t('settings.provider.global.form.notesPlaceholder')}
          value={draft.notes}
          onChange={(e) => onFieldChange('notes', e.target.value)}
          fullWidth
          size="small"
          sx={styles.formField}
        />

        {editingIsCurrent && (
          <Typography sx={styles.currentEditWarning}>
            {t('settings.provider.global.form.currentEditWarning', { app: appLabel })}
          </Typography>
        )}

        {appType === 'claude' ? (
          <>
            <TextField
              select
              label={t('settings.provider.global.form.template')}
              value={templateSelectValue}
              onChange={(e) => {
                const id = e.target.value as ClaudeTemplateId;
                setTemplateSelectValue(id);
                onApplyTemplate(id);
              }}
              fullWidth
              size="small"
              sx={styles.formField}
            >
              {CLAUDE_TEMPLATE_IDS.map((id) => (
                <MenuItem key={id} value={id}>
                  {t(`settings.provider.global.form.templates.${id}`)}
                </MenuItem>
              ))}
            </TextField>
            <JsonEditorField
              label={t('settings.provider.global.form.settingsJson')}
              value={draft.claudeJson}
              onChange={(v) => onFieldChange('claudeJson', v)}
              height={320}
              error={claudeJsonError !== ''}
              helperText={jsonErrorHelper(claudeJsonError) ?? t('settings.provider.global.form.settingsJsonHelper')}
            />
          </>
        ) : (
          <>
            <JsonEditorField
              label={t('settings.provider.global.form.authJson')}
              value={draft.codexAuthJson}
              onChange={(v) => onFieldChange('codexAuthJson', v)}
              height={180}
              error={codexAuthJsonError !== ''}
              helperText={jsonErrorHelper(codexAuthJsonError) ?? t('settings.provider.global.form.authJsonHelper')}
            />
            <JsonEditorField
              label={t('settings.provider.global.form.configToml')}
              value={draft.codexConfigToml}
              onChange={(v) => onFieldChange('codexConfigToml', v)}
              height={260}
              language="ini"
              helperText={t('settings.provider.global.form.configTomlHelper')}
            />
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} sx={{ textTransform: 'none' }}>
          {t('settings.provider.global.form.cancel')}
        </Button>
        <Button onClick={onSubmit} variant="contained" disabled={!canSave} sx={{ textTransform: 'none' }}>
          {t('settings.provider.global.form.save')}
        </Button>
      </DialogActions>
    </Dialog>
  );
};
