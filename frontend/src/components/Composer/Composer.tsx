import { IconButton, TextField, Tooltip, useTheme } from '@mui/material';
import { Box } from '@mui/material';
import SendRoundedIcon from '@mui/icons-material/SendRounded';
import StopRoundedIcon from '@mui/icons-material/StopRounded';
import { useT } from '@/i18n';
import { composerStyles } from './Composer.styles';
import { useComposer } from './useComposer';

export interface ComposerProps {
  sessionId: string;
  // 会话「工作中」（PTY 输出活跃推断）。busy 时仍可发送 —— claude/codex TUI 原生支持
  // 运行中提交入队（steer/queue），按钮 tooltip 提示语义变化；busy 且无草稿时按钮换成
  // 「打断」（红 Stop，发 Esc）—— paseo 发送按钮单按钮多语义状态机的最小移植。
  busy: boolean;
  // 提交整句（父层做两段式 PTY 写入：文本 → 延时 → 单独 '\r'）。reject → 文本回填。
  onSubmit: (text: string) => Promise<void> | void;
  // 打断当前运行（父层写 Esc）。缺省不显示打断按钮。
  onInterrupt?: () => void;
}

// Composer：卡片视图的结构化输入条（paseo Composer 的最小骨架）。多行自适应（到顶内滚）、
// IME 安全 Enter/Shift+Enter、按会话草稿持久化、乐观清空失败回填。附件 / 斜杠命令 /
// @文件补全是后续单元的插槽。
export const Composer = ({ sessionId, busy, onSubmit, onInterrupt }: ComposerProps) => {
  const theme = useTheme();
  const styles = composerStyles(theme);
  const t = useT();
  const vm = useComposer({ sessionId, onSubmit });

  const showInterrupt = busy && !vm.text.trim() && !!onInterrupt;
  const sendLabel = busy ? t('composer.sendQueued') : t('composer.send');

  return (
    <Box sx={styles.root}>
      <TextField
        sx={styles.field}
        value={vm.text}
        onChange={(e) => vm.setText(e.target.value)}
        onKeyDown={vm.onKeyDown}
        placeholder={t('composer.placeholder')}
        multiline
        maxRows={10}
        size="small"
        slotProps={{ htmlInput: { 'aria-label': t('composer.placeholder'), enterKeyHint: 'send' } }}
      />
      {showInterrupt ? (
        <Tooltip title={t('composer.interrupt')}>
          <span>
            <IconButton color="error" onClick={onInterrupt} aria-label={t('composer.interrupt')}>
              <StopRoundedIcon />
            </IconButton>
          </span>
        </Tooltip>
      ) : (
        <Tooltip title={sendLabel}>
          <span>
            <IconButton
              color="primary"
              onClick={vm.submit}
              disabled={!vm.canSend}
              aria-label={sendLabel}
            >
              <SendRoundedIcon />
            </IconButton>
          </span>
        </Tooltip>
      )}
    </Box>
  );
};
