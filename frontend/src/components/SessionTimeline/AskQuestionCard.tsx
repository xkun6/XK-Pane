import { useState } from 'react';
import { Box, Button, Typography, useTheme } from '@mui/material';
import type { SxProps, Theme } from '@mui/material';
import HelpOutlineRoundedIcon from '@mui/icons-material/HelpOutlineRounded';
import CheckCircleRoundedIcon from '@mui/icons-material/CheckCircleRounded';
import CheckRoundedIcon from '@mui/icons-material/CheckRounded';
import CheckBoxRoundedIcon from '@mui/icons-material/CheckBoxRounded';
import CheckBoxOutlineBlankRoundedIcon from '@mui/icons-material/CheckBoxOutlineBlankRounded';
import { useT } from '@/i18n';
import type { TimelineQuestion } from '@/services';
import { sessionTimelineStyles } from './SessionTimeline.styles';
import { buildSingleSelectKeys, buildMultiSelectKeys } from './answerKeys';

export interface AskQuestionCardProps {
  questions: TimelineQuestion[];
  // 仅当这是「最新且待答」的 AskUserQuestion 才允许作答（= 选择器正等待输入）。否则只读渲染：
  // 避免把按键注入一个已结束 / 不在前台的选择器（会污染会话）。父组件按「该事件是否为时间线末尾」判定。
  interactive: boolean;
  onRawInput: (data: string) => void;
}

// 合并若干 sx（落选项过滤）。styles 工厂返回的是纯对象，数组形态由 MUI 运行时支持，这里断言以过类型。
const sx = (...parts: Array<SxProps<Theme> | false | undefined>): SxProps<Theme> =>
  parts.filter(Boolean) as SxProps<Theme>;

// AskQuestionCard：把 AskUserQuestion 渲染成可点选项的问题卡，点选 → 翻译成 PTY 按键驱动选择器作答。
// 多题逐题推进（对齐选择器一次一题）：step 之前=已答（显示选中），step 当前=可作答，step 之后=待答。
export const AskQuestionCard = ({ questions, interactive, onRawInput }: AskQuestionCardProps) => {
  const theme = useTheme();
  const s = sessionTimelineStyles(theme);
  const t = useT();
  const [step, setStep] = useState(0); // 当前待答题序号
  const [answers, setAnswers] = useState<Record<number, number[]>>({}); // qIdx -> 选中项（供显示）
  const [pending, setPending] = useState<number[]>([]); // 当前多选题的本地勾选（未提交）

  const answerSingle = (qIdx: number, optIdx: number, count: number): void => {
    onRawInput(buildSingleSelectKeys(optIdx, count));
    setAnswers((a) => ({ ...a, [qIdx]: [optIdx] }));
    setPending([]);
    setStep(qIdx + 1);
  };
  const toggleMulti = (optIdx: number): void => {
    setPending((p) => (p.includes(optIdx) ? p.filter((i) => i !== optIdx) : [...p, optIdx]));
  };
  const answerMulti = (qIdx: number, count: number): void => {
    if (pending.length === 0) return;
    onRawInput(buildMultiSelectKeys(pending, count));
    setAnswers((a) => ({ ...a, [qIdx]: [...pending].sort((x, y) => x - y) }));
    setPending([]);
    setStep(qIdx + 1);
  };

  const allDone = step >= questions.length;

  return (
    <Box sx={s.askCard}>
      <Box sx={s.askCardHeader}>
        <HelpOutlineRoundedIcon fontSize="small" />
        <Typography component="span" sx={s.askCardHeaderText}>
          {t('sessionTimeline.ask.title')}
        </Typography>
        {!interactive && !allDone ? (
          <Typography component="span" sx={s.askCardHint}>
            {t('sessionTimeline.ask.readonly')}
          </Typography>
        ) : null}
      </Box>

      {questions.map((q, qIdx) => {
        const count = q.options.length;
        const done = qIdx < step;
        const active = interactive && qIdx === step;
        const chosen = answers[qIdx] ?? [];
        return (
          <Box key={qIdx} sx={s.askQ}>
            <Box sx={s.askQHeaderRow}>
              {q.header ? (
                <Box component="span" sx={s.askChip}>
                  {q.header}
                </Box>
              ) : null}
              <Box component="span" sx={s.askMultiHint}>
                {q.multiSelect
                  ? t('sessionTimeline.ask.multiSelectHint')
                  : t('sessionTimeline.ask.singleSelectHint')}
              </Box>
            </Box>
            {q.question ? <Typography sx={s.askQText}>{q.question}</Typography> : null}

            {done ? (
              <Box sx={s.askAnswered}>
                <CheckCircleRoundedIcon fontSize="inherit" />
                <Box component="span">
                  {chosen
                    .map((i) => q.options[i]?.label ?? '')
                    .filter(Boolean)
                    .join('、') || t('sessionTimeline.ask.answered')}
                </Box>
              </Box>
            ) : active && q.multiSelect ? (
              <>
                <Box sx={s.askOptions}>
                  {q.options.map((o, i) => {
                    const sel = pending.includes(i);
                    return (
                      <Box
                        key={i}
                        component="button"
                        type="button"
                        sx={sx(s.askOption, sel && s.askOptionSelected)}
                        onClick={() => toggleMulti(i)}
                      >
                        {sel ? (
                          <CheckBoxRoundedIcon fontSize="small" />
                        ) : (
                          <CheckBoxOutlineBlankRoundedIcon fontSize="small" />
                        )}
                        <Box sx={s.askOptionBody}>
                          <Box sx={s.askOptionLabel}>{o.label}</Box>
                          {o.description ? <Box sx={s.askOptionDesc}>{o.description}</Box> : null}
                        </Box>
                      </Box>
                    );
                  })}
                </Box>
                <Box sx={s.askConfirmRow}>
                  <Button
                    size="small"
                    variant="contained"
                    startIcon={<CheckRoundedIcon />}
                    disabled={pending.length === 0}
                    onClick={() => answerMulti(qIdx, count)}
                  >
                    {t('sessionTimeline.ask.confirm')}
                  </Button>
                </Box>
              </>
            ) : active ? (
              <Box sx={s.askOptions}>
                {q.options.map((o, i) => (
                  <Box
                    key={i}
                    component="button"
                    type="button"
                    sx={s.askOption}
                    onClick={() => answerSingle(qIdx, i, count)}
                  >
                    <Box sx={s.askOptionBody}>
                      <Box sx={s.askOptionLabel}>{o.label}</Box>
                      {o.description ? <Box sx={s.askOptionDesc}>{o.description}</Box> : null}
                    </Box>
                  </Box>
                ))}
              </Box>
            ) : (
              <Box sx={s.askOptions}>
                {q.options.map((o, i) => (
                  <Box key={i} sx={sx(s.askOption, s.askOptionReadonly)}>
                    <Box sx={s.askOptionBody}>
                      <Box sx={s.askOptionLabel}>{o.label}</Box>
                      {o.description ? <Box sx={s.askOptionDesc}>{o.description}</Box> : null}
                    </Box>
                  </Box>
                ))}
                {interactive && qIdx > step ? (
                  <Box sx={s.askWaitingHint}>{t('sessionTimeline.ask.waiting')}</Box>
                ) : null}
              </Box>
            )}
          </Box>
        );
      })}
    </Box>
  );
};
