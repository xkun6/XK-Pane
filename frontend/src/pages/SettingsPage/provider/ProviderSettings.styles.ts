import type { Theme } from '@mui/material';
import type { SystemStyleObject } from '@mui/system';

type S = SystemStyleObject<Theme>;
interface ProviderSettingsSx {
  root: S;
  tabs: S;
  tab: S;
  body: S;
}

export const providerSettingsStyles = (theme: Theme): ProviderSettingsSx => {
  const fp = theme.palette.foundation;
  return {
    root: { display: 'flex', flexDirection: 'column', gap: 2 },
    tabs: { minHeight: 34, flexShrink: 0, borderBottom: `1px solid ${fp.divider}` },
    tab: { minHeight: 34, textTransform: 'none', fontSize: 13 },
    body: { pt: 1 },
  };
};
