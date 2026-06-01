import { useEffect, useMemo, useState } from 'react';
import { Box, Tab, Tabs, useTheme } from '@mui/material';
import { useT } from '@/i18n';
import { mcpeStyles } from './McpePage.styles';
import { useMcpe } from './McpeContext';
import { ConfigPanel } from './ConfigPanel';
import { LaunchPanel } from './LaunchPanel';

type DetailTab = 'config' | 'run';

// 详情：config / run 双 Tab；有未退出的 run 时自动切到 run。
export const PackageDetail = ({ packageId }: { packageId: string }) => {
  const theme = useTheme();
  const styles = mcpeStyles(theme);
  const t = useT();
  const { packages, runs } = useMcpe();
  const pkg = packages.find((p) => p.id === packageId);
  const [tab, setTab] = useState<DetailTab>('config');

  const runRuntime = useMemo(
    () => Object.values(runs).find((r) => r.info.packageId === packageId),
    [runs, packageId],
  );

  useEffect(() => {
    if (runRuntime && !runRuntime.exited && tab === 'config') setTab('run');
  }, [runRuntime, tab]);

  if (!pkg) return null;
  const running = !!runRuntime && !runRuntime.exited;

  return (
    <Box sx={styles.detail}>
      <Tabs
        value={tab}
        onChange={(_, v: DetailTab) => setTab(v)}
        sx={{ minHeight: 34, flexShrink: 0, borderBottom: `1px solid ${theme.palette.foundation.divider}` }}
      >
        <Tab value="config" label={t('mcpe.tab.config')} sx={{ minHeight: 34, textTransform: 'none', fontSize: 12 }} />
        <Tab
          value="run"
          sx={{ minHeight: 34, textTransform: 'none', fontSize: 12 }}
          label={
            <Box component="span" sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              {t('mcpe.tab.run')}
              {running && (
                <Box component="span" sx={{ color: theme.palette.foundation.status.success, fontSize: 10 }}>
                  ●
                </Box>
              )}
            </Box>
          }
        />
      </Tabs>
      <Box sx={styles.detailBody}>
        {tab === 'config' && <ConfigPanel pkg={pkg} />}
        {tab === 'run' && <LaunchPanel pkg={pkg} runtime={runRuntime} />}
      </Box>
    </Box>
  );
};
