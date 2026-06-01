import { Box, Typography, useTheme } from '@mui/material';
import { QRCodeSVG } from 'qrcode.react';
import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import CheckRoundedIcon from '@mui/icons-material/CheckRounded';
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded';
import WifiRoundedIcon from '@mui/icons-material/WifiRounded';
import ErrorOutlineRoundedIcon from '@mui/icons-material/ErrorOutlineRounded';
import { useT } from '@/i18n';
import type { AccessInfo } from '@/services';
import { useWebAccess } from './useWebAccess';
import { webAccessStyles } from './WebAccess.styles';
import { settingsPageStyles } from '../SettingsPage.styles';

// Web 访问子页面（P7 M1）：
//   - 拉后端 webapi.GetAccessInfo → 渲染配对二维码 + 局域网 URL（含一次性 token）
//   - 手机扫码即配对（后端下发 httpOnly cookie），随后与桌面共享会话
//   - 网关未启动 → unavailable 态可重试
//
// 文案 namespace：settings.webaccess.*。
export const WebAccess = () => {
  const theme = useTheme();
  const shared = settingsPageStyles(theme);
  const styles = webAccessStyles(theme);
  const vm = useWebAccess();
  const t = useT();

  return (
    <Box sx={shared.section}>
      <Box sx={shared.sectionHeader}>
        <Typography sx={shared.sectionTitle}>{t('settings.webaccess.title')}</Typography>
        <Typography sx={shared.sectionHint}>{t('settings.webaccess.hint')}</Typography>
      </Box>

      {vm.status.kind === 'loading' && (
        <Box sx={styles.card}>
          <Typography sx={styles.muted}>{t('settings.webaccess.loading')}</Typography>
        </Box>
      )}

      {vm.status.kind === 'unavailable' && (
        <Box sx={styles.card}>
          <Box sx={styles.errorRow}>
            <ErrorOutlineRoundedIcon fontSize="small" />
            <Typography sx={styles.errorText}>
              {t('settings.webaccess.unavailable', { message: vm.status.message })}
            </Typography>
          </Box>
          <Box component="button" type="button" sx={styles.secondaryBtn} onClick={() => void vm.refresh()}>
            <RefreshRoundedIcon fontSize="small" />
            {t('settings.webaccess.retry')}
          </Box>
        </Box>
      )}

      {vm.status.kind === 'ready' && vm.info && (
        <ReadyView info={vm.info} vm={vm} styles={styles} t={t} />
      )}
    </Box>
  );
};

type Vm = ReturnType<typeof useWebAccess>;
type Styles = ReturnType<typeof webAccessStyles>;
type T = ReturnType<typeof useT>;

interface ReadyViewProps {
  info: AccessInfo;
  vm: Vm;
  styles: Styles;
  t: T;
}

const ReadyView = ({ info, vm, styles, t }: ReadyViewProps) => {
  const hasUrl = info.primaryUrl !== '';
  const altUrls = info.urls.filter((u) => u !== info.primaryUrl);

  return (
    <Box sx={styles.card}>
      {hasUrl ? (
        <>
          <Box sx={styles.qrSection}>
            <Box sx={styles.qrBox}>
              <QRCodeSVG value={info.primaryUrl} size={200} level="M" />
            </Box>
            <Typography sx={styles.qrHint}>{t('settings.webaccess.qrHint')}</Typography>
          </Box>

          <UrlRow url={info.primaryUrl} primary vm={vm} styles={styles} t={t} />

          {altUrls.length > 0 && (
            <Box sx={styles.altBlock}>
              <Typography sx={styles.muted}>{t('settings.webaccess.otherUrls')}</Typography>
              {altUrls.map((u) => (
                <UrlRow key={u} url={u} vm={vm} styles={styles} t={t} />
              ))}
            </Box>
          )}
        </>
      ) : (
        <Typography sx={styles.muted}>{t('settings.webaccess.noLan')}</Typography>
      )}

      <Box sx={styles.metaRow}>
        <Typography sx={styles.metaLabel}>{t('settings.webaccess.portLabel')}</Typography>
        <Typography sx={styles.metaValue}>{info.port}</Typography>
      </Box>

      <Box sx={styles.noteBox}>
        <WifiRoundedIcon fontSize="small" />
        <Typography sx={styles.noteText}>{t('settings.webaccess.firewallHint')}</Typography>
      </Box>
      <Typography sx={styles.warnText}>{t('settings.webaccess.tokenWarning')}</Typography>

      <Box component="button" type="button" sx={styles.secondaryBtn} onClick={() => void vm.refresh()}>
        <RefreshRoundedIcon fontSize="small" />
        {t('settings.webaccess.refresh')}
      </Box>
    </Box>
  );
};

interface UrlRowProps {
  url: string;
  primary?: boolean;
  vm: Vm;
  styles: Styles;
  t: T;
}

const UrlRow = ({ url, primary, vm, styles, t }: UrlRowProps) => {
  const copied = vm.copiedUrl === url;
  return (
    <Box sx={styles.urlRow}>
      <Typography sx={primary ? styles.urlTextPrimary : styles.urlText}>{url}</Typography>
      <Box
        component="button"
        type="button"
        sx={styles.copyBtn}
        onClick={() => void vm.copyUrl(url)}
        aria-label={t('settings.webaccess.copyAria')}
      >
        {copied ? <CheckRoundedIcon fontSize="small" /> : <ContentCopyRoundedIcon fontSize="small" />}
        {copied ? t('settings.webaccess.copied') : t('settings.webaccess.copy')}
      </Box>
    </Box>
  );
};
