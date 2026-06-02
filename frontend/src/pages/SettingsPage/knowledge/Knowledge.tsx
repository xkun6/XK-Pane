import { Box, Checkbox, FormControlLabel, LinearProgress, MenuItem, TextField, Typography, useTheme } from '@mui/material';
import type { SxProps, Theme } from '@mui/material';
import LibraryBooksRoundedIcon from '@mui/icons-material/LibraryBooksRounded';
import CloudSyncRoundedIcon from '@mui/icons-material/CloudSyncRounded';
import StopCircleRoundedIcon from '@mui/icons-material/StopCircleRounded';
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded';
import WifiTetheringRoundedIcon from '@mui/icons-material/WifiTetheringRounded';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import Inventory2RoundedIcon from '@mui/icons-material/Inventory2Rounded';
import { useT } from '@/i18n';
import { NativeDialogs, type SyncStatus, type SourceInfo } from '@/services';
import { useKnowledge, ALL_SOURCES, type LogLine, type ProxyForm, type SourcesForm } from './useKnowledge';
import { knowledgeStyles } from './Knowledge.styles';
import { settingsPageStyles } from '../SettingsPage.styles';

// 知识库子页面：
//   1) 状态卡片：是否已 OTA / 占用 / 上次同步 / 各源溯源 + runtime 计数瓦片
//   2) 同步控制：源勾选 + 立即同步 / 取消 + live 进度（阶段 + 抓取计数 + 原始日志）+ 重置（原生确认）
//   3) 代理卡片：URL/Auth + 测试并保存（探测 github.com）
//   4) 源覆盖卡片：mcguide/mconline remote+branch + mcdocs base URL（空 = 官方默认）
//   5) 归档列表（只读）
//
// 文案 namespace：settings.knowledge.*。
export const Knowledge = () => {
  const theme = useTheme();
  const shared = settingsPageStyles(theme);
  const styles = knowledgeStyles(theme);
  const vm = useKnowledge();
  const t = useT();

  const handleReset = async (): Promise<void> => {
    if (vm.syncing || vm.action.kind === 'resetting') return;
    const ok = await NativeDialogs.confirm({
      title: t('settings.knowledge.sync.confirmReset.title'),
      message: t('settings.knowledge.sync.confirmReset.message'),
      okLabel: t('settings.knowledge.sync.confirmReset.ok'),
      cancelLabel: t('settings.knowledge.sync.confirmReset.cancel'),
    });
    if (ok) void vm.resetData();
  };

  return (
    <Box sx={shared.section}>
      <Box sx={shared.sectionHeader}>
        <Typography sx={shared.sectionTitle}>{t('settings.knowledge.title')}</Typography>
        <Typography sx={shared.sectionHint}>{t('settings.knowledge.hint')}</Typography>
      </Box>

      <StatusCard status={vm.status} styles={styles} t={t} />
      <SyncCard vm={vm} styles={styles} t={t} onReset={handleReset} />
      <ProxyCard vm={vm} styles={styles} t={t} />
      <SourcesCard vm={vm} styles={styles} t={t} />
      <ArchivesCard status={vm.status} styles={styles} t={t} />
    </Box>
  );
};

type Vm = ReturnType<typeof useKnowledge>;
type Styles = ReturnType<typeof knowledgeStyles>;
type T = ReturnType<typeof useT>;

// ── 状态卡片 ──────────────────────────────────────────────────────────────

interface StatusCardProps {
  status: SyncStatus | null;
  styles: Styles;
  t: T;
}

const StatusCard = ({ status, styles, t }: StatusCardProps) => {
  const rt = status?.runtimeStats;
  const present = status?.userDataPresent ?? false;
  const manifest = status?.manifest ?? null;
  const sourceEntries: [string, SourceInfo][] = manifest
    ? Object.entries(manifest.sources ?? {}).filter((e): e is [string, SourceInfo] => e[1] !== undefined)
    : [];

  return (
    <Box sx={styles.card}>
      <Box sx={styles.infoRow}>
        <Typography sx={styles.cardTitle}>{t('settings.knowledge.status.title')}</Typography>
        <Box
          component="span"
          sx={[styles.badge, present ? styles.badgePresent : styles.badgeEmbedded] as SxProps<Theme>}
        >
          <LibraryBooksRoundedIcon fontSize="small" />
          {present ? t('settings.knowledge.status.presentBadge') : t('settings.knowledge.status.embeddedBadge')}
        </Box>
      </Box>
      <Typography sx={styles.cardHint}>{t('settings.knowledge.status.hint')}</Typography>

      {/* runtime 计数瓦片：即便从未 OTA 也是内嵌基线的真实数 */}
      <Box sx={styles.statRow}>
        <StatTile value={rt?.apiEntries} label={t('settings.knowledge.status.stats.api')} styles={styles} />
        <StatTile value={rt?.templateEntries} label={t('settings.knowledge.status.stats.templates')} styles={styles} />
        <StatTile value={rt?.guideFiles} label={t('settings.knowledge.status.stats.guides')} styles={styles} />
      </Box>

      <Box sx={styles.infoRow}>
        <Typography sx={styles.infoLabel}>{t('settings.knowledge.status.sizeLabel')}</Typography>
        <Typography sx={styles.infoValue}>{status ? formatMb(status.dataSizeMb) : '…'}</Typography>
      </Box>
      <Box sx={styles.infoRow}>
        <Typography sx={styles.infoLabel}>{t('settings.knowledge.status.lastSyncLabel')}</Typography>
        <Typography sx={styles.infoValue}>
          {manifest?.syncedAt ? manifest.syncedAt : t('settings.knowledge.status.neverSynced')}
        </Typography>
      </Box>

      {sourceEntries.length > 0 && (
        <Box>
          <Typography sx={styles.infoLabel}>{t('settings.knowledge.status.sourcesLabel')}</Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, mt: 0.75 }}>
            {sourceEntries.map(([name, info]) => (
              <SourceProvenance key={name} name={name} info={info} styles={styles} t={t} />
            ))}
          </Box>
        </Box>
      )}
    </Box>
  );
};

interface StatTileProps {
  value: number | undefined;
  label: string;
  styles: Styles;
}

const StatTile = ({ value, label, styles }: StatTileProps) => (
  <Box sx={styles.statTile}>
    <Typography sx={styles.statValue}>{value ?? '—'}</Typography>
    <Typography sx={styles.statLabel}>{label}</Typography>
  </Box>
);

interface SourceProvenanceProps {
  name: string;
  info: SourceInfo;
  styles: Styles;
  t: T;
}

const SourceProvenance = ({ name, info, styles, t }: SourceProvenanceProps) => {
  const kindLabel = t(`settings.knowledge.status.kind.${info.kind}`);
  let meta = '';
  if (info.kind === 'gitea') {
    meta = `${info.remote ?? ''}${info.branch ? `@${info.branch}` : ''}`;
  } else if (info.kind === 'http') {
    meta = info.baseUrl ?? '';
  }
  return (
    <Box sx={styles.sourceProvenanceRow}>
      <Typography component="span" sx={styles.sourceProvenanceName}>
        {name}
      </Typography>
      <Box component="span" sx={[styles.badge, styles.badgeEmbedded] as SxProps<Theme>}>
        {kindLabel}
      </Box>
      {meta && (
        <Typography component="span" sx={styles.sourceProvenanceMeta}>
          {meta}
        </Typography>
      )}
    </Box>
  );
};

// ── 同步控制卡片 ──────────────────────────────────────────────────────────

interface SyncCardProps {
  vm: Vm;
  styles: Styles;
  t: T;
  onReset: () => void;
}

const SyncCard = ({ vm, styles, t, onReset }: SyncCardProps) => {
  const canReset = (vm.status?.userDataPresent ?? false) && !vm.syncing;
  return (
    <Box sx={styles.card}>
      <Typography sx={styles.cardTitle}>{t('settings.knowledge.sync.title')}</Typography>
      <Typography sx={styles.cardHint}>{t('settings.knowledge.sync.hint')}</Typography>

      <Typography sx={styles.infoLabel}>{t('settings.knowledge.sync.sourcesLabel')}</Typography>
      <Box sx={styles.sourcePicker}>
        {ALL_SOURCES.map((s) => (
          <FormControlLabel
            key={s}
            control={
              <Checkbox
                size="small"
                checked={vm.selectedSources.includes(s)}
                disabled={vm.syncing}
                onChange={() => vm.toggleSource(s)}
              />
            }
            label={t(`settings.knowledge.sync.source.${s}`)}
          />
        ))}
      </Box>

      <Box sx={styles.concurrencyRow}>
        <Typography sx={styles.infoLabel}>{t('settings.knowledge.sync.concurrencyLabel')}</Typography>
        <TextField
          select
          size="small"
          value={vm.concurrency}
          disabled={vm.syncing}
          onChange={(e) => vm.setConcurrency(Number(e.target.value))}
          sx={{ width: 96 }}
        >
          {[1, 2, 4, 8, 16].map((n) => (
            <MenuItem key={n} value={n}>
              {n}
            </MenuItem>
          ))}
        </TextField>
        <Typography sx={styles.cardHint}>{t('settings.knowledge.sync.concurrencyHint')}</Typography>
      </Box>

      <Box sx={styles.actions}>
        {!vm.syncing ? (
          <Box
            component="button"
            type="button"
            sx={styles.actionPrimary}
            disabled={vm.selectedSources.length === 0}
            onClick={() => void vm.runSync()}
          >
            <CloudSyncRoundedIcon fontSize="small" />
            {t('settings.knowledge.sync.run')}
          </Box>
        ) : (
          <Box component="button" type="button" sx={styles.actionSecondary} onClick={() => void vm.cancelSync()}>
            <StopCircleRoundedIcon fontSize="small" />
            {t('settings.knowledge.sync.cancel')}
          </Box>
        )}
        <Box
          component="button"
          type="button"
          sx={styles.actionDanger}
          disabled={!canReset}
          onClick={onReset}
        >
          <RestartAltRoundedIcon fontSize="small" />
          {t('settings.knowledge.sync.reset')}
        </Box>
      </Box>

      <ProgressView vm={vm} styles={styles} t={t} />
      <ReportSummary vm={vm} styles={styles} t={t} />

      {vm.action.kind === 'resetting' && (
        <Typography sx={styles.status}>{t('settings.knowledge.feedback.resetting')}</Typography>
      )}
      {vm.action.kind === 'reset' && (
        <Typography sx={styles.statusSuccess}>{t('settings.knowledge.feedback.reset')}</Typography>
      )}
      {vm.action.kind === 'failed' && (
        <Typography sx={styles.statusError}>
          {t('settings.knowledge.feedback.failed', { message: vm.action.message })}
        </Typography>
      )}
    </Box>
  );
};

interface ProgressViewProps {
  vm: Vm;
  styles: Styles;
  t: T;
}

const ProgressView = ({ vm, styles, t }: ProgressViewProps) => {
  const { progress, syncing, logLines } = vm;
  if (!syncing && !progress && logLines.length === 0) return null;

  const stage = progress?.stage ?? (syncing ? 'started' : '');
  const isHttpFetch = progress?.stage === 'http_fetch' && (progress.total ?? 0) > 0;
  const pct = isHttpFetch ? Math.min(100, ((progress?.fetched ?? 0) / (progress?.total ?? 1)) * 100) : 0;

  return (
    <Box sx={styles.progressBlock}>
      {stage && (
        <Box sx={styles.progressHeadline}>
          <span>{t(`settings.knowledge.progress.stage.${stage}`)}</span>
          {isHttpFetch && (
            <Typography component="span" sx={styles.progressCount}>
              {progress?.fetched ?? 0}/{progress?.total ?? 0}
            </Typography>
          )}
          {progress?.stage === 'build_index' && (
            <Typography component="span" sx={styles.progressCount}>
              {progress.kind} · {progress.entries ?? 0}
            </Typography>
          )}
        </Box>
      )}
      {syncing &&
        (isHttpFetch ? (
          <LinearProgress variant="determinate" value={pct} />
        ) : (
          <LinearProgress variant="indeterminate" />
        ))}

      {logLines.length > 0 && (
        <Box sx={styles.logBox}>
          {logLines.map((line, idx) => (
            <LogRow key={idx} line={line} styles={styles} />
          ))}
        </Box>
      )}
    </Box>
  );
};

interface LogRowProps {
  line: LogLine;
  styles: Styles;
}

const LogRow = ({ line, styles }: LogRowProps) => {
  const tone =
    line.level === 'error' ? styles.logError : line.level === 'warn' ? styles.logWarn : undefined;
  return (
    <Typography sx={[styles.logLine, tone].filter(Boolean) as SxProps<Theme>}>{line.msg}</Typography>
  );
};

interface ReportSummaryProps {
  vm: Vm;
  styles: Styles;
  t: T;
}

const ReportSummary = ({ vm, styles, t }: ReportSummaryProps) => {
  const r = vm.lastReport;
  if (!r) return null;
  const synced = r.sourcesSynced ?? [];
  const skipped = r.sourcesSkipped ?? [];
  const partial = r.partialErrors ?? [];
  const failures = r.failures ?? [];
  return (
    <Box sx={styles.reportBlock}>
      <Box sx={styles.reportRow}>
        <Box component="span" sx={styles.reportChip}>
          {synced.length > 0
            ? t('settings.knowledge.sync.report.synced', { list: synced.join(', ') })
            : t('settings.knowledge.sync.report.none')}
        </Box>
        {skipped.length > 0 && (
          <Box component="span" sx={styles.reportChip}>
            {t('settings.knowledge.sync.report.skipped', { list: skipped.map((s) => s.source).join(', ') })}
          </Box>
        )}
        {partial.length > 0 && (
          <Box component="span" sx={styles.reportChip}>
            {t('settings.knowledge.sync.report.partial', { count: partial.length })}
          </Box>
        )}
      </Box>
      {/* 部分失败详情：后端 partialErrors 原句（verbatim 诊断文本，同日志区不走 t()）。 */}
      {partial.length > 0 && (
        <Box sx={styles.partialList}>
          <Typography sx={styles.partialTitle}>
            {t('settings.knowledge.sync.report.partialDetail')}
          </Typography>
          {partial.map((msg, idx) => (
            <Typography key={idx} sx={[styles.logLine, styles.logWarn] as SxProps<Theme>}>
              {msg}
            </Typography>
          ))}
        </Box>
      )}
      {/* 逐文件失败明细：source 标签 + 路径 + 原始错误，精确定位是哪个文件失败。 */}
      {failures.length > 0 && (
        <Box sx={styles.failureBox}>
          <Typography sx={styles.partialTitle}>
            {t('settings.knowledge.sync.report.failuresDetail', { count: failures.length })}
          </Typography>
          {failures.map((f, idx) => (
            <Box key={idx} sx={styles.failureRow}>
              <Typography sx={styles.failurePath}>
                {f.source} · {f.path}
              </Typography>
              <Typography sx={styles.failureError}>{f.error}</Typography>
            </Box>
          ))}
        </Box>
      )}
    </Box>
  );
};

// ── 代理卡片 ──────────────────────────────────────────────────────────────

const ProxyCard = ({ vm, styles, t }: { vm: Vm; styles: Styles; t: T }) => {
  const form: ProxyForm = vm.proxyForm;
  const testing = vm.action.kind === 'testingProxy';
  return (
    <Box sx={styles.card}>
      <Typography sx={styles.cardTitle}>{t('settings.knowledge.proxy.title')}</Typography>
      <Typography sx={styles.cardHint}>{t('settings.knowledge.proxy.hint')}</Typography>
      <Box sx={styles.formGrid}>
        <TextField
          size="small"
          fullWidth
          label={t('settings.knowledge.proxy.urlLabel')}
          placeholder={t('settings.knowledge.proxy.urlPlaceholder')}
          value={form.url}
          onChange={(e) => vm.setProxyField('url', e.target.value)}
        />
        <TextField
          size="small"
          fullWidth
          label={t('settings.knowledge.proxy.authLabel')}
          placeholder={t('settings.knowledge.proxy.authPlaceholder')}
          value={form.auth}
          onChange={(e) => vm.setProxyField('auth', e.target.value)}
        />
      </Box>
      <Box sx={styles.actions}>
        <Box
          component="button"
          type="button"
          sx={styles.actionSecondary}
          disabled={testing}
          onClick={() => void vm.testAndSaveProxy()}
        >
          <WifiTetheringRoundedIcon fontSize="small" />
          {testing ? t('settings.knowledge.proxy.testing') : t('settings.knowledge.proxy.test')}
        </Box>
      </Box>
      {vm.action.kind === 'proxyOk' && (
        <Typography sx={styles.statusSuccess}>
          {t('settings.knowledge.proxy.ok', { ms: vm.action.latencyMs })}
        </Typography>
      )}
      {vm.action.kind === 'proxyFail' && (
        <Typography sx={styles.statusError}>{t('settings.knowledge.proxy.fail')}</Typography>
      )}
    </Box>
  );
};

// ── 源覆盖卡片 ────────────────────────────────────────────────────────────

const SourcesCard = ({ vm, styles, t }: { vm: Vm; styles: Styles; t: T }) => {
  const form: SourcesForm = vm.sourcesForm;
  const saving = vm.action.kind === 'savingSources';
  const ph = t('settings.knowledge.sources.placeholderDefault');
  return (
    <Box sx={styles.card}>
      <Typography sx={styles.cardTitle}>{t('settings.knowledge.sources.title')}</Typography>
      <Typography sx={styles.cardHint}>{t('settings.knowledge.sources.hint')}</Typography>
      <Box sx={styles.formGrid}>
        <TextField
          size="small"
          fullWidth
          label={t('settings.knowledge.sources.mcguideRemote')}
          placeholder={ph}
          value={form.mcguideRemote}
          onChange={(e) => vm.setSourcesField('mcguideRemote', e.target.value)}
        />
        <TextField
          size="small"
          fullWidth
          label={t('settings.knowledge.sources.mcguideBranch')}
          placeholder={ph}
          value={form.mcguideBranch}
          onChange={(e) => vm.setSourcesField('mcguideBranch', e.target.value)}
        />
        <TextField
          size="small"
          fullWidth
          label={t('settings.knowledge.sources.mconlineRemote')}
          placeholder={ph}
          value={form.mconlineRemote}
          onChange={(e) => vm.setSourcesField('mconlineRemote', e.target.value)}
        />
        <TextField
          size="small"
          fullWidth
          label={t('settings.knowledge.sources.mconlineBranch')}
          placeholder={ph}
          value={form.mconlineBranch}
          onChange={(e) => vm.setSourcesField('mconlineBranch', e.target.value)}
        />
        <TextField
          size="small"
          fullWidth
          sx={styles.formFull}
          label={t('settings.knowledge.sources.mcdocsBaseUrl')}
          placeholder={ph}
          value={form.mcdocsBaseUrl}
          onChange={(e) => vm.setSourcesField('mcdocsBaseUrl', e.target.value)}
        />
      </Box>
      <Box sx={styles.actions}>
        <Box
          component="button"
          type="button"
          sx={styles.actionSecondary}
          disabled={saving}
          onClick={() => void vm.saveSources()}
        >
          <SaveRoundedIcon fontSize="small" />
          {saving ? t('settings.knowledge.sources.saving') : t('settings.knowledge.sources.save')}
        </Box>
      </Box>
      {vm.action.kind === 'savedSources' && (
        <Typography sx={styles.statusSuccess}>{t('settings.knowledge.sources.saved')}</Typography>
      )}
    </Box>
  );
};

// ── 归档列表 ──────────────────────────────────────────────────────────────

const ArchivesCard = ({ status, styles, t }: { status: SyncStatus | null; styles: Styles; t: T }) => {
  const archives = status?.archives ?? [];
  return (
    <Box sx={styles.card}>
      <Typography sx={styles.cardTitle}>{t('settings.knowledge.archives.title')}</Typography>
      {archives.length === 0 ? (
        <Typography sx={styles.cardHint}>{t('settings.knowledge.archives.empty')}</Typography>
      ) : (
        <Box sx={styles.archiveList}>
          {archives.map((a) => (
            <Box key={a.name} sx={styles.archiveRow}>
              <Inventory2RoundedIcon fontSize="small" />
              <Box sx={styles.archiveMain}>
                <Typography sx={styles.archiveName}>{a.name}</Typography>
                <Typography sx={styles.archiveMeta}>
                  {t('settings.knowledge.archives.meta', { created: a.createdAt, size: formatMb(a.sizeMb) })}
                </Typography>
              </Box>
            </Box>
          ))}
        </Box>
      )}
    </Box>
  );
};

// MB 格式化（后端已给 MB 浮点）。
const formatMb = (mb: number): string => {
  if (!Number.isFinite(mb) || mb <= 0) return '0 MB';
  if (mb < 0.1) return `${(mb * 1024).toFixed(0)} KB`;
  if (mb < 1024) return `${mb.toFixed(1)} MB`;
  return `${(mb / 1024).toFixed(2)} GB`;
};
