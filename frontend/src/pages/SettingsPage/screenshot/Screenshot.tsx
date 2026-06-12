import { type ChangeEvent } from 'react';
import { Box, Typography, useTheme } from '@mui/material';
import type { SxProps, Theme } from '@mui/material';
import KeyboardRoundedIcon from '@mui/icons-material/KeyboardRounded';
import CheckRoundedIcon from '@mui/icons-material/CheckRounded';
import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import { useT } from '@/i18n';
import { useScreenshot, type ScreenshotStatus } from './useScreenshot';
import { screenshotStyles } from './Screenshot.styles';
import { settingsPageStyles } from '../SettingsPage.styles';

// 截图子页面（参照 cc-pane ScreenshotSection）：
//   1) 快捷键录制框：聚焦进入录制，按组合键 → 待应用，应用 = 后端重注册 + 落库
//   2) 保留天数：写时清理阈值（0 = 永久保留）
// 文案 namespace：settings.screenshot.*。
export const Screenshot = () => {
  const theme = useTheme();
  const shared = settingsPageStyles(theme);
  const styles = screenshotStyles(theme);
  const vm = useScreenshot();
  const t = useT();

  const dirty = vm.pending !== null && vm.pending !== vm.shortcut;
  const recorderText =
    vm.recording && vm.pending === null
      ? t('settings.screenshot.shortcut.recording')
      : (vm.pending ?? vm.shortcut);

  return (
    <Box sx={shared.section}>
      <Box sx={shared.sectionHeader}>
        <Typography sx={shared.sectionTitle}>{t('settings.screenshot.title')}</Typography>
        <Typography sx={shared.sectionHint}>{t('settings.screenshot.hint')}</Typography>
      </Box>

      {/* —— 快捷键 —— */}
      <Box sx={styles.card}>
        <Box sx={styles.row}>
          <Typography sx={styles.label}>{t('settings.screenshot.shortcut.label')}</Typography>
          <Box
            component="button"
            type="button"
            sx={
              vm.recording
                ? ([styles.recorder, styles.recorderRecording] as SxProps<Theme>)
                : styles.recorder
            }
            onFocus={vm.startRecording}
            onBlur={vm.stopRecording}
            onKeyDown={vm.onRecordKey}
            aria-label={t('settings.screenshot.shortcut.ariaRecorder')}
          >
            <KeyboardRoundedIcon fontSize="small" />
            {recorderText}
          </Box>
          {dirty && (
            <>
              <Box
                component="button"
                type="button"
                sx={styles.actionPrimary}
                disabled={vm.status.kind === 'applying'}
                onClick={() => void vm.applyShortcut()}
              >
                <CheckRoundedIcon fontSize="small" />
                {t('settings.screenshot.shortcut.apply')}
              </Box>
              <Box
                component="button"
                type="button"
                sx={styles.actionSecondary}
                disabled={vm.status.kind === 'applying'}
                onClick={vm.cancelPending}
              >
                <CloseRoundedIcon fontSize="small" />
                {t('settings.screenshot.shortcut.cancel')}
              </Box>
            </>
          )}
        </Box>
        <Typography sx={styles.hint}>{t('settings.screenshot.shortcut.hint')}</Typography>
        <StatusLine status={vm.status} styles={styles} t={t} />
      </Box>

      {/* —— 保留天数 —— */}
      <Box sx={styles.card}>
        <Box sx={styles.row}>
          <Typography sx={styles.label}>{t('settings.screenshot.retention.label')}</Typography>
          <Box
            component="input"
            type="number"
            min={0}
            value={vm.retentionDraft}
            onChange={(e: ChangeEvent<HTMLInputElement>) => vm.setRetentionDraft(e.target.value)}
            onBlur={vm.commitRetention}
            sx={styles.numberInput}
            aria-label={t('settings.screenshot.retention.label')}
          />
        </Box>
        <Typography sx={styles.hint}>{t('settings.screenshot.retention.hint')}</Typography>
      </Box>
    </Box>
  );
};

type Styles = ReturnType<typeof screenshotStyles>;
type T = ReturnType<typeof useT>;

interface StatusLineProps {
  status: ScreenshotStatus;
  styles: Styles;
  t: T;
}

const StatusLine = ({ status, styles, t }: StatusLineProps) => {
  switch (status.kind) {
    case 'idle':
      return null;
    case 'applying':
      return (
        <Typography sx={styles.status}>{t('settings.screenshot.feedback.applying')}</Typography>
      );
    case 'applied':
      return (
        <Typography sx={styles.status}>{t('settings.screenshot.feedback.applied')}</Typography>
      );
    case 'failed':
      return (
        <Typography sx={styles.statusError}>
          {t('settings.screenshot.feedback.failed', { message: status.message })}
        </Typography>
      );
  }
};
