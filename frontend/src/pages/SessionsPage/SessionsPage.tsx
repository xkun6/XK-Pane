import { useMemo, useState } from 'react';
import { Box, IconButton, Tooltip, useTheme } from '@mui/material';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import ChevronRightRoundedIcon from '@mui/icons-material/ChevronRightRounded';
import DeleteSweepRoundedIcon from '@mui/icons-material/DeleteSweepRounded';
import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import { useT } from '@/i18n';
import { sessionsPageStyles } from './SessionsPage.styles';
import { useSessionsPage } from './useSessionsPage';
import { groupByWorkspace } from './groupLaunches';
import { ResumeDetailPopover } from './ResumeDetailPopover';

// SessionsPage：移植 cc-pane SessionsView —— 活跃会话（点击聚焦终端页）+ 最近启动按工作空间
// 分组折叠（点击恢复会话 + 详情气泡 + 删除/清空）。落在原 history 路由（用户拍板替换 Local History）。
export const SessionsPage = () => {
  const theme = useTheme();
  const styles = sessionsPageStyles(theme);
  const t = useT();
  const vm = useSessionsPage();

  const ungroupedLabel = t('sessions.ungrouped');
  const groups = useMemo(
    () => groupByWorkspace(vm.launchHistory, ungroupedLabel),
    [vm.launchHistory, ungroupedLabel],
  );

  // 折叠状态：默认全部展开（collapsed 存已折叠的组名）。组件本地态（禁 localStorage）。
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const toggleGroup = (name: string): void =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });

  const detailStrings = {
    detail: t('sessions.detail'),
    sessionId: t('sessions.sessionId'),
    launchTime: t('sessions.launchTime'),
    lastPrompt: t('sessions.lastPrompt'),
    copyId: t('sessions.copyId'),
    resume: t('sessions.resume'),
    delete: t('sessions.deleteRecord'),
  };

  const onClear = (): void =>
    void vm.clearHistory({
      confirmTitle: t('sessions.clear.confirmTitle'),
      confirmMessage: t('sessions.clear.confirmMessage'),
      okLabel: t('sessions.clear.ok'),
      cancelLabel: t('sessions.clear.cancel'),
      errorTitle: t('sessions.error.title'),
    });

  return (
    <Box sx={styles.root}>
      <Box sx={styles.header}>
        <Box sx={styles.eyebrow}>{t('sessions.eyebrow')}</Box>
        <Box sx={styles.title}>{t('sessions.title')}</Box>
      </Box>

      {/* 活跃会话 */}
      {vm.activeSessions.length > 0 && (
        <Box sx={styles.section}>
          <Box sx={styles.sectionTitle}>
            {t('sessions.active', { count: String(vm.activeSessions.length) })}
          </Box>
          {vm.activeSessions.map((s) => (
            <Box
              key={s.tabId}
              sx={styles.activeRow}
              role="button"
              tabIndex={0}
              onClick={() => vm.focusSession(s.paneId, s.tabId)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  vm.focusSession(s.paneId, s.tabId);
                }
              }}
            >
              <Box component="span" sx={styles.activeIconWrap}>
                <TerminalRoundedIcon fontSize="small" />
                <Box
                  component="span"
                  sx={{
                    ...styles.activeDot,
                    ...(s.status === 'active' ? styles.activeDotOn : styles.activeDotIdle),
                  }}
                />
              </Box>
              <Box sx={styles.activeTitle}>{s.title || t('sessions.untitled')}</Box>
            </Box>
          ))}
        </Box>
      )}

      {/* 最近启动 */}
      <Box sx={styles.section}>
        <Box sx={styles.sectionHeader}>
          <Box sx={styles.sectionTitle}>{t('sessions.recent')}</Box>
          {vm.launchHistory.length > 0 && (
            <Tooltip title={t('sessions.clear.action')}>
              <IconButton size="small" aria-label={t('sessions.clear.action')} onClick={onClear}>
                <DeleteSweepRoundedIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          )}
        </Box>

        {groups.length === 0 ? (
          <Box sx={styles.empty}>{t('sessions.noResumable')}</Box>
        ) : (
          groups.map((group) => {
            const isCollapsed = collapsed.has(group.workspaceName);
            return (
              <Box key={group.workspaceName}>
                <Box
                  sx={styles.groupHeader}
                  role="button"
                  tabIndex={0}
                  aria-expanded={!isCollapsed}
                  onClick={() => toggleGroup(group.workspaceName)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      toggleGroup(group.workspaceName);
                    }
                  }}
                >
                  <ChevronRightRoundedIcon
                    sx={{ fontSize: 16, transform: isCollapsed ? 'none' : 'rotate(90deg)' }}
                  />
                  <Box sx={styles.groupName}>{group.workspaceName}</Box>
                  <Box component="span" sx={styles.countPill}>
                    {group.records.length}
                  </Box>
                </Box>

                {!isCollapsed &&
                  group.records.map((record) => (
                    <Box
                      key={record.id}
                      sx={styles.launchRow}
                      role="button"
                      tabIndex={0}
                      onClick={() => vm.resume(record)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          vm.resume(record);
                        }
                      }}
                    >
                      <PlayArrowRoundedIcon
                        fontSize="small"
                        sx={{ color: theme.palette.foundation.status.success, flexShrink: 0 }}
                      />
                      <Box sx={styles.launchMain}>
                        <Box sx={styles.projectName}>{record.projectName}</Box>
                        <Box sx={styles.sessionId}>{record.resumeSessionId?.slice(0, 8)}…</Box>
                        {record.lastPrompt && (
                          <Box sx={styles.lastPrompt}>{record.lastPrompt.slice(0, 40)}</Box>
                        )}
                      </Box>
                      <Box
                        sx={styles.launchMeta}
                        onClick={(e) => e.stopPropagation()}
                        onKeyDown={(e) => e.stopPropagation()}
                        role="presentation"
                      >
                        <Box component="span" sx={styles.time}>
                          {formatRelativeTime(record.launchedAt, t)}
                        </Box>
                        <Box className="sessions-row-actions" sx={styles.rowActions}>
                          <ResumeDetailPopover
                            record={record}
                            styles={styles}
                            strings={detailStrings}
                            onResume={vm.resume}
                            onDelete={vm.deleteRecord}
                          />
                          <Tooltip title={t('sessions.deleteRecord')}>
                            <IconButton
                              size="small"
                              aria-label={t('sessions.deleteRecord')}
                              onClick={() => void vm.deleteRecord(record.id)}
                            >
                              <CloseRoundedIcon sx={{ fontSize: 16 }} />
                            </IconButton>
                          </Tooltip>
                        </Box>
                      </Box>
                    </Box>
                  ))}
              </Box>
            );
          })
        )}
      </Box>
    </Box>
  );
};

// 相对时间（数据格式化，按粒度走 t() 复数键）。
function formatRelativeTime(ts: string, t: (k: string, p?: Record<string, string>) => string): string {
  if (!ts) return '';
  const then = new Date(ts).getTime();
  if (Number.isNaN(then)) return ts;
  const sec = Math.max(0, Math.floor((Date.now() - then) / 1000));
  if (sec < 60) return t('sessions.time.justNow');
  const min = Math.floor(sec / 60);
  if (min < 60) return t('sessions.time.minutes', { n: String(min) });
  const hr = Math.floor(min / 60);
  if (hr < 24) return t('sessions.time.hours', { n: String(hr) });
  const day = Math.floor(hr / 24);
  return t('sessions.time.days', { n: String(day) });
}
