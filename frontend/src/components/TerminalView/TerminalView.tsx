import { useState } from 'react';
import { Box, IconButton, Tooltip, useTheme } from '@mui/material';
import ChatRoundedIcon from '@mui/icons-material/ChatRounded';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import '@xterm/xterm/css/xterm.css';
import { TerminalService } from '@/services';
import { lifecycleRole } from '@/services/transport';
import { useT } from '@/i18n';
import { TerminalKeyBar } from '@/components/TerminalKeyBar';
import { SessionTimeline } from '@/components/SessionTimeline';
import { terminalViewStyles } from './TerminalView.styles';
import { useTerminalView, type UseTerminalViewArgs } from './useTerminalView';

export type TerminalViewProps = UseTerminalViewArgs;

// TerminalView：把一个后端 PTY 会话渲染为可交互终端（xterm）。
// 视图只负责挂载 host 容器；全部 xterm 生命周期 / I/O 接线在 useTerminalView。
// viewer（Web/移动，P7 M6）：终端下方加虚拟键栏（Esc/Tab/Ctrl+C/方向键），补齐手机软键盘缺失键。
// 结构化视图（手机首选）：claude 会话默认覆盖一层 SessionTimeline（从 JSONL 转录渲染可重排卡片，
// 解决「PTY 网格在窄屏挤成几列」的根本问题）。xterm 在底下持续运行、零干扰；点「终端」收起覆盖层、
// 点 FAB 重新唤起。非 claude / 加载中时 SessionTimeline 自渲染为 null → 露出 xterm 镜像。
export const TerminalView = (props: TerminalViewProps) => {
  const theme = useTheme();
  const styles = terminalViewStyles(theme);
  const t = useT();
  const { hostRef } = useTerminalView(props);
  const [showTimeline, setShowTimeline] = useState(true);
  // SessionTimeline 上报：该会话是否有结构化转录（claude/reclaude）。只有 true 才显示「切换视图」按钮，
  // 非 claude（supported=false / 加载中 null）只露终端、无卡片可切。
  const [supported, setSupported] = useState<boolean | null>(null);

  if (lifecycleRole === 'viewer') {
    const sid = props.sessionId;
    return (
      <Box sx={styles.viewerRoot}>
        <Box ref={hostRef} sx={styles.viewerHost} />
        <TerminalKeyBar
          disabled={!sid}
          onKey={(data) => {
            if (sid) void TerminalService.write(sid, data).catch(() => {});
          }}
        />
        {sid && (
          <SessionTimeline
            sessionId={sid}
            visible={showTimeline}
            onSend={(text) => {
              void TerminalService.write(sid, `${text}\r`).catch(() => {});
            }}
            onSupportedChange={setSupported}
          />
        )}
        {sid && supported === true && (
          <Tooltip title={showTimeline ? t('sessionTimeline.showTerminal') : t('sessionTimeline.showChat')}>
            <IconButton
              size="small"
              sx={styles.viewToggle}
              aria-label={showTimeline ? t('sessionTimeline.showTerminal') : t('sessionTimeline.showChat')}
              onClick={() => setShowTimeline((v) => !v)}
            >
              {showTimeline ? <TerminalRoundedIcon fontSize="small" /> : <ChatRoundedIcon fontSize="small" />}
            </IconButton>
          </Tooltip>
        )}
      </Box>
    );
  }

  return <Box ref={hostRef} sx={styles.host} />;
};
