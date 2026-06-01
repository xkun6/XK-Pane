import { useEffect, useState, type ReactNode } from 'react';
import {
  Box,
  Button,
  Checkbox,
  FormControlLabel,
  IconButton,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import FolderSpecialRoundedIcon from '@mui/icons-material/FolderSpecialRounded';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import DownloadRoundedIcon from '@mui/icons-material/DownloadRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import { useT } from '@/i18n';
import { NativeDialogs, MCPEService, type MCPackage } from '@/services';
import { mcpeStyles } from './McpePage.styles';
import { useMcpe } from './McpeContext';
import { messageOf } from './useMcpePage';

// Field 必须定义在模块顶层（不可内联在组件体内，否则每次重渲染换组件类型 → Input 失焦）。
const Field = ({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) => {
  const theme = useTheme();
  const styles = mcpeStyles(theme);
  return (
    <Box sx={styles.field}>
      <Typography component="label" sx={styles.fieldLabel}>
        {label}
      </Typography>
      {children}
      {hint && <Typography sx={styles.fieldHint}>{hint}</Typography>}
    </Box>
  );
};

const WORLD_BOOLS: Array<{ key: 'enableCheats' | 'keepInventory' | 'doDaylightCycle' | 'doWeatherCycle'; labelKey: string }> = [
  { key: 'enableCheats', labelKey: 'mcpe.world.enableCheats' },
  { key: 'keepInventory', labelKey: 'mcpe.world.keepInventory' },
  { key: 'doDaylightCycle', labelKey: 'mcpe.world.doDaylightCycle' },
  { key: 'doWeatherCycle', labelKey: 'mcpe.world.doWeatherCycle' },
];

const numberSx = { '& .MuiInputBase-input': { fontSize: 12, py: 0.5 } };

export const ConfigPanel = ({ pkg }: { pkg: MCPackage }) => {
  const theme = useTheme();
  const styles = mcpeStyles(theme);
  const t = useT();
  const { updatePackage, launch, moveWorldToTrash, scanLaunchers } = useMcpe();

  const [local, setLocal] = useState<MCPackage>(pkg);
  const [busy, setBusy] = useState(false);
  const [scanned, setScanned] = useState<string[]>([]);
  const [bridgePort, setBridgePort] = useState<number | null>(null);
  const [bridgeConnected, setBridgeConnected] = useState(false);
  const [bridgeAddr, setBridgeAddr] = useState<string | null>(null);

  useEffect(() => setLocal(pkg), [pkg]);

  // 桥状态：仅启用桥时拉一次（非轮询；launch 后切到本面板会自然 remount 重拉）。
  useEffect(() => {
    if (!local.debugOptions.includeTestMod) {
      setBridgePort(null);
      return;
    }
    let cancelled = false;
    MCPEService.getBridgeStatus()
      .then((s) => {
        if (cancelled) return;
        setBridgePort(s.port);
        setBridgeConnected(s.connected);
        setBridgeAddr(s.remoteAddr ?? null);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [local.debugOptions.includeTestMod, pkg.id]);

  const dirty = JSON.stringify(local) !== JSON.stringify(pkg);

  const patchWorld = (patch: Partial<MCPackage['world']>) => setLocal((l) => ({ ...l, world: { ...l.world, ...patch } }));
  const patchDebug = (patch: Partial<MCPackage['debugOptions']>) =>
    setLocal((l) => ({ ...l, debugOptions: { ...l.debugOptions, ...patch } }));

  const fail = async (e: unknown) => {
    await NativeDialogs.error(t('mcpe.error.title'), messageOf(e));
  };

  const save = async () => {
    setBusy(true);
    try {
      await updatePackage(local);
    } catch (e) {
      await fail(e);
    } finally {
      setBusy(false);
    }
  };

  const doLaunch = async () => {
    setBusy(true);
    try {
      if (dirty) await updatePackage(local);
      await launch(pkg.id);
    } catch (e) {
      await fail(e);
    } finally {
      setBusy(false);
    }
  };

  const pickLauncher = async () => {
    const f = await NativeDialogs.openFile({
      title: t('mcpe.config.pickLauncher'),
      filters: [{ displayName: t('mcpe.config.exeFilter'), pattern: '*.exe' }],
    });
    if (f) setLocal((l) => ({ ...l, gameExecutablePath: f }));
  };

  const doScan = async () => {
    try {
      setScanned(await scanLaunchers());
    } catch (e) {
      await fail(e);
    }
  };

  const addIncludedDir = async () => {
    const dirs = await NativeDialogs.openFiles({ title: t('mcpe.config.pickModDir') }); // 注：多选目录暂用 openFiles
    if (!dirs || dirs.length === 0) return;
    setLocal((l) => ({ ...l, includedModDirs: Array.from(new Set([...l.includedModDirs, ...dirs])) }));
  };

  const removeIncludedDir = (dir: string) =>
    setLocal((l) => ({ ...l, includedModDirs: l.includedModDirs.filter((d) => d !== dir) }));

  const importWorld = async () => {
    const picked = await NativeDialogs.openDirectory({ title: t('mcpe.world.importTitle') });
    if (!picked) return;
    const seg = picked.split(/[\\/]/).filter(Boolean).pop() ?? 'imported-world';
    patchWorld({ sourceDir: picked, folderName: seg });
  };

  const doTrashWorld = async () => {
    const ok = await NativeDialogs.confirm({
      title: t('mcpe.world.trashTitle'),
      message: t('mcpe.world.trashMessage', { folder: pkg.world.folderName }),
      okLabel: t('mcpe.world.trashOk'),
      cancelLabel: t('mcpe.delete.cancel'),
    });
    if (!ok) return;
    try {
      await moveWorldToTrash(pkg.id);
    } catch (e) {
      await fail(e);
    }
  };

  return (
    <Box sx={styles.form}>
      {/* ① 操作栏 */}
      <Box sx={{ display: 'flex', gap: 1, mb: 1.5 }}>
        <Button
          size="small"
          variant="contained"
          startIcon={<PlayArrowRoundedIcon />}
          disabled={busy}
          onClick={() => void doLaunch()}
          sx={{ textTransform: 'none' }}
        >
          {dirty ? t('mcpe.action.saveAndLaunch') : t('mcpe.action.launch')}
        </Button>
        <Button
          size="small"
          variant="outlined"
          startIcon={<SaveRoundedIcon />}
          disabled={!dirty || busy}
          onClick={() => void save()}
          sx={{ textTransform: 'none' }}
        >
          {t('mcpe.action.save')}
        </Button>
      </Box>

      {/* ② 名称 */}
      <Field label={t('mcpe.config.name')}>
        <TextField
          size="small"
          value={local.name}
          onChange={(e) => setLocal((l) => ({ ...l, name: e.target.value }))}
          sx={numberSx}
        />
      </Field>

      {/* ③ 项目目录（只读） */}
      <Field label={t('mcpe.config.projectDir')} hint={t('mcpe.config.projectDirHint')}>
        <Box sx={styles.readonlyPath}>{local.projectPath}</Box>
      </Field>

      {/* ④ 启动器路径 */}
      <Field label={t('mcpe.config.launcher')} hint={t('mcpe.config.launcherHint')}>
        <Box sx={{ display: 'flex', gap: 0.5 }}>
          <TextField
            size="small"
            value={local.gameExecutablePath ?? ''}
            placeholder={t('mcpe.config.launcherPlaceholder')}
            onChange={(e) => setLocal((l) => ({ ...l, gameExecutablePath: e.target.value }))}
            sx={{ ...numberSx, flex: 1 }}
          />
          <Tooltip title={t('mcpe.config.browse')}>
            <IconButton size="small" aria-label={t('mcpe.config.browse')} onClick={() => void pickLauncher()}>
              <FolderOpenRoundedIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Tooltip title={t('mcpe.config.scan')}>
            <IconButton size="small" aria-label={t('mcpe.config.scan')} onClick={() => void doScan()}>
              <FolderSpecialRoundedIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
        {scanned.length > 0 && (
          <Box sx={styles.scanList}>
            {scanned.map((p) => (
              <Box
                component="button"
                key={p}
                sx={styles.scanItem}
                onClick={() => {
                  setLocal((l) => ({ ...l, gameExecutablePath: p }));
                  setScanned([]);
                }}
              >
                {p}
              </Box>
            ))}
          </Box>
        )}
      </Field>

      {/* ⑤ 附加组件目录 */}
      <Field label={t('mcpe.config.includedDirs')} hint={t('mcpe.config.includedDirsHint')}>
        {local.includedModDirs.length === 0 ? (
          <Typography sx={{ ...styles.fieldHint, fontStyle: 'italic' }}>{t('mcpe.config.none')}</Typography>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {local.includedModDirs.map((d) => (
              <Box key={d} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                <Box sx={{ ...styles.readonlyPath, flex: 1, fontSize: 10, padding: '4px 6px' }}>{d}</Box>
                <IconButton size="small" aria-label={t('mcpe.action.delete')} onClick={() => removeIncludedDir(d)}>
                  <DeleteOutlineRoundedIcon sx={{ fontSize: 14 }} />
                </IconButton>
              </Box>
            ))}
          </Box>
        )}
        <Button
          size="small"
          variant="outlined"
          startIcon={<AddRoundedIcon />}
          onClick={() => void addIncludedDir()}
          sx={{ textTransform: 'none', mt: 0.5, alignSelf: 'flex-start' }}
        >
          {t('mcpe.config.addDir')}
        </Button>
      </Field>

      {/* ⑥ 世界设置 */}
      <Box sx={styles.divider} />
      <Typography sx={styles.sectionTitle}>{t('mcpe.world.section')}</Typography>

      {/* ⑦ 存档目录名 */}
      <Field label={t('mcpe.world.folderName')} hint={t('mcpe.world.folderNameHint')}>
        <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}>
          <TextField
            size="small"
            value={local.world.folderName}
            onChange={(e) => patchWorld({ folderName: e.target.value })}
            sx={{ ...numberSx, flex: 1, '& .MuiInputBase-input': { fontFamily: 'monospace', fontSize: 11, py: 0.5 } }}
          />
          <Tooltip title={t('mcpe.world.importHint')}>
            <IconButton size="small" aria-label={t('mcpe.world.import')} onClick={() => void importWorld()}>
              <DownloadRoundedIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Tooltip title={t('mcpe.world.trashHint')}>
            <IconButton size="small" aria-label={t('mcpe.world.trash')} onClick={() => void doTrashWorld()}>
              <DeleteOutlineRoundedIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
        {local.world.sourceDir && (
          <>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mt: 0.5 }}>
              <Typography sx={styles.fieldHint}>{t('mcpe.world.source')}</Typography>
              <Box sx={{ ...styles.readonlyPath, flex: 1, fontSize: 10, padding: '2px 6px' }}>{local.world.sourceDir}</Box>
              <Tooltip title={t('mcpe.world.clearSource')}>
                <IconButton
                  size="small"
                  aria-label={t('mcpe.world.clearSource')}
                  onClick={() => patchWorld({ sourceDir: '', resetOnLaunch: false })}
                >
                  <CloseRoundedIcon sx={{ fontSize: 14 }} />
                </IconButton>
              </Tooltip>
            </Box>
            <FormControlLabel
              control={
                <Checkbox
                  size="small"
                  checked={!!local.world.resetOnLaunch}
                  onChange={(e) => patchWorld({ resetOnLaunch: e.target.checked })}
                />
              }
              label={<Typography sx={{ fontSize: 11 }}>{t('mcpe.world.resetOnLaunch')}</Typography>}
            />
          </>
        )}
      </Field>

      {/* ⑧⑨ 种子 + 用户名 */}
      <Field label={t('mcpe.world.seed')}>
        <TextField
          size="small"
          type="number"
          value={local.world.seed}
          onChange={(e) => patchWorld({ seed: Number(e.target.value) || 0 })}
          sx={numberSx}
        />
      </Field>
      <Field label={t('mcpe.world.userName')}>
        <TextField
          size="small"
          value={local.world.userName}
          onChange={(e) => patchWorld({ userName: e.target.value })}
          sx={numberSx}
        />
      </Field>

      {/* ⑩ 模式 + 类型 */}
      <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1 }}>
        <Field label={t('mcpe.world.gameMode')}>
          <TextField
            size="small"
            select
            value={local.world.gameMode}
            onChange={(e) => patchWorld({ gameMode: e.target.value as MCPackage['world']['gameMode'] })}
            sx={numberSx}
          >
            <MenuItem value="creative">{t('mcpe.world.modeCreative')}</MenuItem>
            <MenuItem value="survival">{t('mcpe.world.modeSurvival')}</MenuItem>
          </TextField>
        </Field>
        <Field label={t('mcpe.world.levelType')}>
          <TextField
            size="small"
            select
            value={local.world.levelType}
            onChange={(e) => patchWorld({ levelType: e.target.value as MCPackage['world']['levelType'] })}
            sx={numberSx}
          >
            <MenuItem value="default">{t('mcpe.world.typeDefault')}</MenuItem>
            <MenuItem value="flat">{t('mcpe.world.typeFlat')}</MenuItem>
          </TextField>
        </Field>
      </Box>

      {/* ⑪ 四布尔 */}
      <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 0, mt: 1 }}>
        {WORLD_BOOLS.map(({ key, labelKey }) => (
          <FormControlLabel
            key={key}
            control={
              <Checkbox size="small" checked={!!local.world[key]} onChange={(e) => patchWorld({ [key]: e.target.checked })} />
            }
            label={<Typography sx={{ fontSize: 11 }}>{t(labelKey)}</Typography>}
          />
        ))}
      </Box>

      {/* ⑫ 调试热键 */}
      <Box sx={styles.divider} />
      <Typography sx={styles.sectionTitle}>{t('mcpe.debug.section')}</Typography>
      <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1 }}>
        <Field label={t('mcpe.debug.reloadKey')} hint={t('mcpe.debug.reloadKeyHint')}>
          <TextField
            size="small"
            type="number"
            value={local.debugOptions.reloadKey}
            onChange={(e) => patchDebug({ reloadKey: Number(e.target.value) || 0 })}
            sx={numberSx}
          />
        </Field>
        <Field label={t('mcpe.debug.reloadWorldKey')} hint={t('mcpe.debug.reloadWorldKeyHint')}>
          <TextField
            size="small"
            type="number"
            value={local.debugOptions.reloadWorldKey}
            onChange={(e) => patchDebug({ reloadWorldKey: Number(e.target.value) || 0 })}
            sx={numberSx}
          />
        </Field>
      </Box>
      <FormControlLabel
        control={
          <Checkbox
            size="small"
            checked={local.debugOptions.reloadKeyGlobal}
            onChange={(e) => patchDebug({ reloadKeyGlobal: e.target.checked })}
          />
        }
        label={<Typography sx={{ fontSize: 11 }}>{t('mcpe.debug.reloadKeyGlobal')}</Typography>}
      />

      {/* ⑮ 启用 MCDK 调试桥 */}
      <Box sx={{ mt: 1.5 }}>
        <FormControlLabel
          control={
            <Checkbox
              size="small"
              checked={local.debugOptions.includeTestMod}
              onChange={(e) => patchDebug({ includeTestMod: e.target.checked })}
            />
          }
          label={<Typography sx={{ fontSize: 11 }}>{t('mcpe.debug.includeTestMod')}</Typography>}
        />
        <Typography sx={{ ...styles.fieldHint, ml: 3.5 }}>{t('mcpe.debug.includeTestModHint')}</Typography>
        {local.debugOptions.includeTestMod && bridgePort !== null && (
          <Box sx={styles.bridgeLine}>
            {t('mcpe.debug.bridgeListening', { port: String(bridgePort) })} ·{' '}
            {bridgeConnected
              ? t('mcpe.debug.bridgeConnected', { addr: bridgeAddr ?? '?' })
              : t('mcpe.debug.bridgeWaiting')}
          </Box>
        )}
      </Box>
    </Box>
  );
};
