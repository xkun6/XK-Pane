import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface CliVersionBarSx {
  root: S;
  icon: S;
  info: S;
  titleLine: S;
  name: S;
  chipUpdateAvailable: S;
  chipUpToDate: S;
  chipNotInstalled: S;
  subLine: S;
  actions: S;
  upgradeBtn: S;
}

export const cliVersionBarStyles = (theme: Theme): CliVersionBarSx => {
  const fp = theme.palette.foundation;
  return {
    root: {
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      px: 2,
      py: 1.25,
      mb: 2,
      maxWidth: 760,
      borderRadius: 1,
      border: `1px solid ${fp.divider}`,
      backgroundColor: fp.bg.surface,
    },
    icon: { color: fp.text.muted, flexShrink: 0 },
    info: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0.5 },
    titleLine: { display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 },
    name: { fontSize: 14, fontWeight: 600, color: fp.text.primary },
    chipUpdateAvailable: { fontSize: 11, height: 20, color: fp.status.warning, borderColor: fp.status.warning },
    chipUpToDate: { fontSize: 11, height: 20, color: fp.status.success, borderColor: fp.status.success },
    chipNotInstalled: { fontSize: 11, height: 20, color: fp.text.muted, borderColor: fp.divider },
    subLine: {
      fontSize: 12,
      color: fp.text.muted,
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    actions: { display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 },
    upgradeBtn: { textTransform: 'none', borderRadius: 1.5 },
  };
};
