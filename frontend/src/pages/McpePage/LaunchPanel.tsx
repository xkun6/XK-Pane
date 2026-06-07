import { useEffect, useMemo, useRef, useState } from 'react';
import { Box, Button, Link, Tooltip, Typography, useTheme } from '@mui/material';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import StopRoundedIcon from '@mui/icons-material/StopRounded';
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded';
import BoltRoundedIcon from '@mui/icons-material/BoltRounded';
import PublicRoundedIcon from '@mui/icons-material/PublicRounded';
import { useT } from '@/i18n';
import { MCPEService, NativeDialogs, type MCPackage } from '@/services';
import { copyText } from '@/shared/clipboard';
import { mcpeStyles, severityColor } from './McpePage.styles';
import { useMcpe, type MCRunRuntime, type MCOutputLine } from './McpeContext';
import { messageOf } from './useMcpePage';
import { resolvePath, shortTs, tokenize, type Segment } from './launchPanelFormat';

const BRIDGE_POLL_MS = 2000;

// 桥状态徽章（presentational）：轮询由 LaunchPanel 统一持有，与「热重载」按钮共享 connected 态。
// 定义在模块顶层（避免内联换组件类型）。
const BridgeStatusBadge = ({ port, connected }: { port: number; connected: boolean }) => {
  const theme = useTheme();
  const styles = mcpeStyles(theme);
  const t = useT();
  return (
    <Box sx={styles.bridgeBadge}>
      <Box
        sx={{
          ...styles.bridgeDot,
          backgroundColor: connected ? theme.palette.foundation.status.success : theme.palette.foundation.status.warning,
        }}
      />
      <Typography component="span" sx={{ fontSize: 10, color: theme.palette.foundation.text.muted }}>
        {t('mcpe.run.bridgeBadge', { port: String(port) })}{' '}
        {connected ? t('mcpe.run.bridgeConnected') : t('mcpe.run.bridgeWaiting')}
      </Typography>
    </Box>
  );
};

const segCache = new WeakMap<MCOutputLine, Segment[]>();

export const LaunchPanel = ({ pkg, runtime }: { pkg: MCPackage; runtime?: MCRunRuntime }) => {
  const theme = useTheme();
  const styles = mcpeStyles(theme);
  const t = useT();
  const { launch, stop, restart } = useMcpe();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [restarting, setRestarting] = useState(false);
  const [bridge, setBridge] = useState<{ port: number | null; connected: boolean }>({ port: null, connected: false });

  const running = !!runtime && !runtime.exited;
  // 桥仅在「运行中 + 启用调试桥」时有意义；据此驱动徽章 + 热重载按钮的可用态。
  const bridgeEnabled = running && pkg.debugOptions.includeTestMod;

  const fail = async (e: unknown) => {
    await NativeDialogs.error(t('mcpe.error.title'), messageOf(e));
  };

  const handleLaunch = async () => {
    try {
      await launch(pkg.id);
    } catch (e) {
      await fail(e);
    }
  };
  const handleStop = async () => {
    try {
      await stop(pkg.id);
    } catch (e) {
      await fail(e);
    }
  };
  const handleRestart = async () => {
    if (restarting) return;
    setRestarting(true);
    try {
      await restart(pkg.id);
    } catch (e) {
      await fail(e);
    } finally {
      setRestarting(false);
    }
  };

  // XK 无内置代码编辑器 → 点击 file:line 退化为复制解析后的绝对路径（带提示）。
  const handleLink = (p: string) => {
    void copyText(resolvePath(pkg.projectPath, p));
  };

  const output = runtime?.output;
  const tokenized = useMemo(() => {
    if (!output) return [];
    return output.map((line) => {
      let segs = segCache.get(line);
      if (!segs) {
        segs = tokenize(line.text);
        segCache.set(line, segs);
      }
      return { line, segs };
    });
  }, [output]);

  useEffect(() => {
    if (containerRef.current) containerRef.current.scrollTop = containerRef.current.scrollHeight;
  }, [output?.length]);

  // 桥状态 2s 轮询（桥是全局单例）；仅在 bridgeEnabled 时拉，否则复位，避免空转。
  useEffect(() => {
    if (!bridgeEnabled) {
      setBridge({ port: null, connected: false });
      return;
    }
    let alive = true;
    const fetchStatus = () => {
      MCPEService.getBridgeStatus()
        .then((s) => {
          if (alive) setBridge({ port: s.port, connected: s.connected });
        })
        .catch(() => {});
    };
    fetchStatus();
    const timer = setInterval(fetchStatus, BRIDGE_POLL_MS);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [bridgeEnabled]);

  // 经桥热重载（fire-and-forget）；桥未连入时按钮已禁用，仅极端竞态下才会抛错 → 弹原生错误框。
  const handleReloadScript = async () => {
    try {
      await MCPEService.reloadScript();
    } catch (e) {
      await fail(e);
    }
  };
  const handleReloadWorld = async () => {
    try {
      await MCPEService.reloadWorld();
    } catch (e) {
      await fail(e);
    }
  };

  const pidText = runtime?.info.pid != null ? String(runtime.info.pid) : '?';

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Box sx={styles.launchToolbar}>
        {!running ? (
          <Button
            size="small"
            variant="contained"
            startIcon={<PlayArrowRoundedIcon />}
            onClick={() => void handleLaunch()}
            sx={{ textTransform: 'none', flexShrink: 0 }}
          >
            {t('mcpe.action.launch')}
          </Button>
        ) : (
          <>
            <Button
              size="small"
              variant="contained"
              color="error"
              startIcon={<StopRoundedIcon />}
              disabled={restarting}
              onClick={() => void handleStop()}
              sx={{ textTransform: 'none', flexShrink: 0 }}
            >
              {t('mcpe.action.stop')}
            </Button>
            <Button
              size="small"
              variant="outlined"
              startIcon={<RestartAltRoundedIcon sx={restarting ? { animation: 'mcpe-spin 1s linear infinite' } : undefined} />}
              disabled={restarting}
              onClick={() => void handleRestart()}
              sx={{ textTransform: 'none', flexShrink: 0, '@keyframes mcpe-spin': { to: { transform: 'rotate(360deg)' } } }}
            >
              {restarting ? t('mcpe.action.restarting') : t('mcpe.action.restart')}
            </Button>
            {pkg.debugOptions.includeTestMod && (
              <>
                <Tooltip title={bridge.connected ? t('mcpe.action.reloadScriptHint') : t('mcpe.action.bridgeRequired')}>
                  {/* span 包裹：disabled 按钮自身不触发 Tooltip */}
                  <Box component="span" sx={{ flexShrink: 0 }}>
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<BoltRoundedIcon />}
                      disabled={!bridge.connected || restarting}
                      onClick={() => void handleReloadScript()}
                      sx={{ textTransform: 'none', flexShrink: 0 }}
                    >
                      {t('mcpe.action.reloadScript')}
                    </Button>
                  </Box>
                </Tooltip>
                <Tooltip title={bridge.connected ? t('mcpe.action.reloadWorldHint') : t('mcpe.action.bridgeRequired')}>
                  <Box component="span" sx={{ flexShrink: 0 }}>
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<PublicRoundedIcon />}
                      disabled={!bridge.connected || restarting}
                      onClick={() => void handleReloadWorld()}
                      sx={{ textTransform: 'none', flexShrink: 0 }}
                    >
                      {t('mcpe.action.reloadWorld')}
                    </Button>
                  </Box>
                </Tooltip>
              </>
            )}
          </>
        )}

        {runtime && (
          <Typography sx={styles.pidInfo}>
            {runtime.exited
              ? t('mcpe.run.exited', { pid: pidText, code: String(runtime.exited.code) })
              : t('mcpe.run.running', { pid: pidText })}
          </Typography>
        )}

        {bridgeEnabled && bridge.port !== null && (
          <BridgeStatusBadge port={bridge.port} connected={bridge.connected} />
        )}
      </Box>

      <Box ref={containerRef} sx={styles.logArea}>
        {tokenized.length > 0 ? (
          tokenized.map(({ line, segs }, i) => (
            <Box key={i} sx={{ color: severityColor(theme, line.severity) }}>
              {line.timestamp && (
                <Box
                  component="span"
                  sx={{ fontSize: 9, color: theme.palette.foundation.text.muted, mr: 0.5 }}
                >
                  {shortTs(line.timestamp)}
                </Box>
              )}
              {line.category && (
                <Box component="span" sx={{ fontSize: 9, color: theme.palette.foundation.accent, mr: 0.5 }}>
                  [{line.category}]
                </Box>
              )}
              {segs.map((seg, j) =>
                seg.link ? (
                  <Tooltip key={j} title={t('mcpe.run.copyPath')}>
                    <Link
                      component="button"
                      type="button"
                      underline="hover"
                      sx={{
                        color: 'inherit',
                        textDecorationStyle: 'dotted',
                        cursor: 'pointer',
                        font: 'inherit',
                        verticalAlign: 'baseline',
                        '&:hover': { color: theme.palette.foundation.accent },
                      }}
                      onClick={() => seg.link && handleLink(seg.link.path)}
                    >
                      {seg.text}
                    </Link>
                  </Tooltip>
                ) : (
                  <Box component="span" key={j}>
                    {seg.text}
                  </Box>
                ),
              )}
            </Box>
          ))
        ) : (
          <Box sx={styles.logEmpty}>
            {runtime
              ? runtime.exited
                ? t('mcpe.run.logExited')
                : t('mcpe.run.logWaiting')
              : t('mcpe.run.logIdle')}
          </Box>
        )}
      </Box>
    </Box>
  );
};
