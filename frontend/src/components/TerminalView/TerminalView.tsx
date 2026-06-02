import { Box, useTheme } from '@mui/material';
import '@xterm/xterm/css/xterm.css';
import { TerminalService } from '@/services';
import { lifecycleRole } from '@/services/transport';
import { TerminalKeyBar } from '@/components/TerminalKeyBar';
import { SessionTimeline } from '@/components/SessionTimeline';
import type { TabViewMode } from '@/terminal';
import { terminalViewStyles } from './TerminalView.styles';
import { useTerminalView, type UseTerminalViewArgs } from './useTerminalView';

export interface TerminalViewProps extends UseTerminalViewArgs {
  /** 视图模式（来自 Tab，缺省 = 'terminal'）：'card' 时叠加 SessionTimeline 结构化对话覆盖层。 */
  viewMode?: TabViewMode;
}

// TerminalView：把一个后端 PTY 会话渲染为可交互终端（xterm）。
// 视图只负责挂载 host 容器；全部 xterm 生命周期 / I/O 接线在 useTerminalView。
// viewer（Web/移动，P7 M6）：终端下方加虚拟键栏（Esc/Tab/Ctrl+C/方向键），补齐手机软键盘缺失键。
// 卡片视图（claude/reclaude 会话）：viewMode==='card' 时覆盖一层 SessionTimeline（从 JSONL 转录渲染可
// 重排卡片，桌面 owner 经 transcriptsvc binding 取数、Web viewer 经 /api/sessions/timeline）。xterm 在底下
// 持续运行、零干扰。视图切换控件已上移到 TabBar（取代原右上角浮动按钮 + 「新建」+），由 viewMode 驱动。
// SessionTimeline 始终挂载（!visible 返回 null 但保留 hook 已累积事件），切回卡片即时复现、不从头重拉。
export const TerminalView = (props: TerminalViewProps) => {
  const theme = useTheme();
  const styles = terminalViewStyles(theme);
  const { hostRef } = useTerminalView(props);
  const sid = props.sessionId;
  const viewMode = props.viewMode ?? 'terminal';
  // 仅 claude/reclaude 有结构化转录；非此类会话不挂 SessionTimeline（避免无谓轮询，且无卡片可切）。
  const canCard = props.cliTool === 'claude' || props.cliTool === 'reclaude';
  const cardVisible = canCard && viewMode === 'card';

  const timeline =
    canCard && sid ? (
      <SessionTimeline
        sessionId={sid}
        visible={cardVisible}
        onSend={(text) => {
          void TerminalService.write(sid, `${text}\r`).catch(() => {});
        }}
      />
    ) : null;

  if (lifecycleRole === 'viewer') {
    return (
      <Box sx={styles.viewerRoot}>
        <Box ref={hostRef} sx={styles.viewerHost} />
        <TerminalKeyBar
          disabled={!sid}
          onKey={(data) => {
            if (sid) void TerminalService.write(sid, data).catch(() => {});
          }}
        />
        {timeline}
      </Box>
    );
  }

  // 桌面（owner）：相对定位外壳，host 跑 xterm，timeline 作覆盖层叠在其上（卡片态可见）。
  return (
    <Box sx={styles.ownerRoot}>
      <Box ref={hostRef} sx={styles.host} />
      {timeline}
    </Box>
  );
};
