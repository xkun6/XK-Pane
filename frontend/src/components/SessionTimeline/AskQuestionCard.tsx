import { useState } from 'react';
import { Box, Button, Typography, useTheme } from '@mui/material';
import type { SxProps, Theme } from '@mui/material';
import HelpOutlineRoundedIcon from '@mui/icons-material/HelpOutlineRounded';
import SendRoundedIcon from '@mui/icons-material/SendRounded';
import HourglassEmptyRoundedIcon from '@mui/icons-material/HourglassEmptyRounded';
import RadioButtonCheckedRoundedIcon from '@mui/icons-material/RadioButtonCheckedRounded';
import RadioButtonUncheckedRoundedIcon from '@mui/icons-material/RadioButtonUncheckedRounded';
import CheckBoxRoundedIcon from '@mui/icons-material/CheckBoxRounded';
import CheckBoxOutlineBlankRoundedIcon from '@mui/icons-material/CheckBoxOutlineBlankRounded';
import { useT } from '@/i18n';
import type { TimelineQuestion } from '@/services';
import { sessionTimelineStyles } from './SessionTimeline.styles';
import { buildAnswerChunks } from './answerKeys';

export interface AskQuestionCardProps {
  questions: TimelineQuestion[];
  // 仅当这是「最新且待答」的 AskUserQuestion 才允许作答（= 选择器正等待输入）。否则只读渲染（历史卡）：
  // 避免把按键注入一个已结束 / 不在前台的选择器（会污染会话）。父组件按「是否 pending 待答」判定。
  interactive: boolean;
  onRawInput: (data: string) => void;
}

// 合并若干 sx（落选项过滤）。styles 工厂返回的是纯对象，数组形态由 MUI 运行时支持，这里断言以过类型。
const sx = (...parts: Array<SxProps<Theme> | false | undefined>): SxProps<Theme> =>
  parts.filter(Boolean) as SxProps<Theme>;

// 段间延时(ms)：选择器每题 tab 独立挂载，发完一段（含切到下一 tab）后等其挂载，再发下一段，防丢键。
const CHUNK_DELAY_MS = 120;

// AskQuestionCard：把 AskUserQuestion 渲染成「表单式」问题卡 —— 所有问题同屏可选（单选=单选钮、多选=复选框），
// 全部答完后点底部「提交」一次性把所有选择翻译成 PTY 按键序列驱动选择器作答（对齐选择器逐题推进、末题 Enter 提交）。
export const AskQuestionCard = ({ questions, interactive, onRawInput }: AskQuestionCardProps) => {
  const theme = useTheme();
  const s = sessionTimelineStyles(theme);
  const t = useT();
  const [selections, setSelections] = useState<Record<number, number[]>>({}); // qIdx -> 选中项下标
  const [submitted, setSubmitted] = useState(false); // 已提交 → 锁定卡片，等待转录的只读历史卡接管

  const pick = (qIdx: number, optIdx: number, multi: boolean): void => {
    if (submitted) return;
    setSelections((prev) => {
      const cur = prev[qIdx] ?? [];
      if (multi) {
        const next = cur.includes(optIdx) ? cur.filter((i) => i !== optIdx) : [...cur, optIdx];
        return { ...prev, [qIdx]: next };
      }
      return { ...prev, [qIdx]: [optIdx] }; // 单选：替换
    });
  };

  // 每题至少选一项才允许提交（单选必有 1；多选要求 ≥1）。
  const allAnswered = questions.every((_, qIdx) => (selections[qIdx]?.length ?? 0) > 0);

  const submit = async (): Promise<void> => {
    if (!allAnswered || submitted) return;
    setSubmitted(true);
    // 逐段写入、段间延时：选择器多题是 tab 式（→ 切题、单选 Enter 自动前进、多选 Space 切换），
    // 每题 tab 独立挂载，一次性猛灌会在切 tab 瞬间丢键（详见 answerKeys.ts）。
    const chunks = buildAnswerChunks(questions, selections);
    for (let i = 0; i < chunks.length; i++) {
      const chunk = chunks[i];
      if (chunk) onRawInput(chunk);
      if (i < chunks.length - 1) await new Promise<void>((r) => setTimeout(r, CHUNK_DELAY_MS));
    }
  };

  return (
    <Box sx={s.askCard}>
      <Box sx={s.askCardHeader}>
        <HelpOutlineRoundedIcon fontSize="small" />
        <Typography component="span" sx={s.askCardHeaderText}>
          {t('sessionTimeline.ask.title')}
        </Typography>
        {!interactive ? (
          <Typography component="span" sx={s.askCardHint}>
            {t('sessionTimeline.ask.readonly')}
          </Typography>
        ) : null}
      </Box>

      {questions.map((q, qIdx) => {
        const multi = !!q.multiSelect;
        const locked = !interactive || submitted; // 只读历史卡 or 已提交 → 选项不可点
        return (
          <Box key={qIdx} sx={s.askQ}>
            <Box sx={s.askQHeaderRow}>
              {q.header ? (
                <Box component="span" sx={s.askChip}>
                  {q.header}
                </Box>
              ) : null}
              <Box component="span" sx={s.askMultiHint}>
                {multi
                  ? t('sessionTimeline.ask.multiSelectHint')
                  : t('sessionTimeline.ask.singleSelectHint')}
              </Box>
            </Box>
            {q.question ? <Typography sx={s.askQText}>{q.question}</Typography> : null}

            <Box sx={s.askOptions}>
              {q.options.map((o, i) => {
                const sel = (selections[qIdx] ?? []).includes(i);
                const SelIcon = multi
                  ? sel
                    ? CheckBoxRoundedIcon
                    : CheckBoxOutlineBlankRoundedIcon
                  : sel
                    ? RadioButtonCheckedRoundedIcon
                    : RadioButtonUncheckedRoundedIcon;
                const body = (
                  <>
                    {/* 只读历史卡不含选中项（转录无），不显示选择控件，避免「空单选圈」误导 */}
                    {interactive ? (
                      <SelIcon fontSize="small" sx={sx(s.askOptionTick, sel && s.askOptionTickOn)} />
                    ) : null}
                    <Box sx={s.askOptionBody}>
                      <Box sx={s.askOptionLabel}>{o.label}</Box>
                      {o.description ? <Box sx={s.askOptionDesc}>{o.description}</Box> : null}
                    </Box>
                  </>
                );
                if (locked) {
                  return (
                    <Box key={i} sx={sx(s.askOption, s.askOptionReadonly, sel && s.askOptionSelected)}>
                      {body}
                    </Box>
                  );
                }
                return (
                  <Box
                    key={i}
                    component="button"
                    type="button"
                    sx={sx(s.askOption, sel && s.askOptionSelected)}
                    onClick={() => pick(qIdx, i, multi)}
                  >
                    {body}
                  </Box>
                );
              })}
            </Box>
          </Box>
        );
      })}

      {interactive ? (
        <Box sx={s.askConfirmRow}>
          {submitted ? (
            <Box sx={s.askWaitingHint}>
              <HourglassEmptyRoundedIcon fontSize="inherit" />
              {t('sessionTimeline.ask.submitted')}
            </Box>
          ) : (
            <Button
              size="small"
              variant="contained"
              startIcon={<SendRoundedIcon />}
              disabled={!allAnswered}
              onClick={() => void submit()}
            >
              {t('sessionTimeline.ask.submit')}
            </Button>
          )}
        </Box>
      ) : null}
    </Box>
  );
};
