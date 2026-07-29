import { Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography, useTheme } from '@mui/material';
import { useT } from '@/i18n';
import { JsonEditorField } from '@/components/JsonEditorField';
import type { AgentAppType } from '@/services';
import type { JsonFieldError } from './agentProviderJson';
import { globalProviderPanelStyles } from './GlobalProviderPanel.styles';

export interface SystemConfigDialogProps {
  appType: AgentAppType;
  open: boolean;
  value: string;
  error: JsonFieldError;
  onChange: (value: string) => void;
  onCancel: () => void;
  onSave: () => void;
}

// 系统配置基底编辑对话框（View，无业务）：claude 两层模型的公共层，
// 与 GlobalProviderFormDialog 编辑的「供应商凭据补丁」相对——保存后由后端与当前
// 供应商补丁深合并写入 live 配置文件。
export const SystemConfigDialog = ({ appType, open, value, error, onChange, onCancel, onSave }: SystemConfigDialogProps) => {
  const theme = useTheme();
  const styles = globalProviderPanelStyles(theme);
  const t = useT();

  const appLabel = t(`settings.provider.tabs.${appType}`);

  const jsonErrorHelper = (code: JsonFieldError): string | undefined => {
    if (code === 'empty') return t('settings.provider.global.form.jsonEmptyError');
    if (code === 'invalid') return t('settings.provider.global.form.jsonInvalidError');
    return undefined;
  };

  return (
    <Dialog open={open} onClose={onCancel} maxWidth="md" fullWidth>
      <DialogTitle>{t('settings.provider.global.baseConfig.dialogTitle', { app: appLabel })}</DialogTitle>
      <DialogContent>
        <Typography sx={styles.currentEditWarning}>
          {t('settings.provider.global.baseConfig.dialogHint', { app: appLabel })}
        </Typography>
        <JsonEditorField
          label={t('settings.provider.global.baseConfig.jsonLabel')}
          value={value}
          onChange={onChange}
          height={360}
          error={error !== ''}
          helperText={jsonErrorHelper(error)}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} sx={{ textTransform: 'none' }}>
          {t('settings.provider.global.form.cancel')}
        </Button>
        <Button onClick={onSave} variant="contained" disabled={error !== ''} sx={{ textTransform: 'none' }}>
          {t('settings.provider.global.form.save')}
        </Button>
      </DialogActions>
    </Dialog>
  );
};
