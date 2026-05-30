import { Box, Typography, useTheme } from '@mui/material';
import { alpha } from '@mui/material/styles';
import type { ReactNode } from 'react';
import type { DiffHunk, DiffLine, DiffResult, InlineChange } from '@/services';
import { diffViewStyles } from './DiffView.styles';

// DiffView：纯展示组件，渲染后端 DiffResult（行级 hunk + 字符级 inline）。
// 不持 i18n（占位文案由 labels prop 传入，保持组件无依赖、可复用）；颜色取 theme.palette.foundation。
// inline 高亮按 **码点** 切片（InlineChange.start/end 是 rune 下标，非 UTF-16 单元）。

export interface DiffViewLabels {
  binary: string; // 二进制文件，无法 diff
  truncated: string; // 文件过大，已截断
  identical: string; // 无差异
  added: string; // "+{{n}}" 的可读形式由调用方插值；这里仅作 aria/title 备用
  removed: string;
}

export interface DiffViewProps {
  diff: DiffResult;
  labels: DiffViewLabels;
}

// 把一行内容按 inline 变更区间（码点下标）切成 [普通, 高亮, 普通...] 段。
function renderContent(content: string, inlines: InlineChange[] | undefined, hlColor: string): ReactNode {
  if (!inlines || inlines.length === 0) return content;
  const cps = Array.from(content); // 按码点（rune）拆分，与后端 rune 下标对齐
  const sorted = [...inlines].sort((a, b) => a.start - b.start);
  const segs: ReactNode[] = [];
  let pos = 0;
  sorted.forEach((ic, i) => {
    const start = Math.max(pos, ic.start);
    if (start > pos) segs.push(cps.slice(pos, start).join(''));
    const end = Math.max(start, ic.end);
    if (end > start) {
      segs.push(
        <Box key={`hl-${i}`} component="span" sx={{ backgroundColor: hlColor, borderRadius: '2px' }}>
          {cps.slice(start, end).join('')}
        </Box>,
      );
    }
    pos = Math.max(pos, end);
  });
  if (pos < cps.length) segs.push(cps.slice(pos).join(''));
  return segs;
}

export const DiffView = ({ diff, labels }: DiffViewProps) => {
  const theme = useTheme();
  const styles = diffViewStyles(theme);

  if (diff.isBinary) return <Box sx={styles.root}><Box sx={styles.message}>{labels.binary}</Box></Box>;
  if (diff.truncated) return <Box sx={styles.root}><Box sx={styles.message}>{labels.truncated}</Box></Box>;
  if (diff.hunks.length === 0) return <Box sx={styles.root}><Box sx={styles.message}>{labels.identical}</Box></Box>;

  const success = theme.palette.foundation.status.success;
  const danger = theme.palette.foundation.status.danger;

  const lineBg = (t: DiffLine['changeType']): string | undefined => {
    if (t === 'insert') return alpha(success, 0.14);
    if (t === 'delete') return alpha(danger, 0.14);
    return undefined;
  };
  const inlineHl = (t: DiffLine['changeType']): string => (t === 'insert' ? alpha(success, 0.38) : alpha(danger, 0.38));
  const sign = (t: DiffLine['changeType']): string => (t === 'insert' ? '+' : t === 'delete' ? '-' : ' ');

  return (
    <Box sx={styles.root}>
      <Box sx={styles.stats}>
        <Typography component="span" sx={styles.statAdd}>
          +{diff.stats.additions}
        </Typography>
        <Typography component="span" sx={styles.statDel}>
          −{diff.stats.deletions}
        </Typography>
      </Box>
      {diff.hunks.map((hunk: DiffHunk, hi) => (
        <Box key={hi}>
          <Box sx={styles.hunkHeader}>
            {`@@ -${hunk.oldStart},${hunk.oldCount} +${hunk.newStart},${hunk.newCount} @@`}
          </Box>
          {hunk.lines.map((line, li) => (
            <Box key={li} sx={{ ...styles.line, backgroundColor: lineBg(line.changeType) }}>
              <Box component="span" sx={styles.gutter}>
                {line.oldLineNo ?? ''}
              </Box>
              <Box component="span" sx={styles.gutter}>
                {line.newLineNo ?? ''}
              </Box>
              <Box component="span" sx={styles.sign}>
                {sign(line.changeType)}
              </Box>
              <Box component="span" sx={styles.content}>
                {renderContent(line.content, line.inlineChanges, inlineHl(line.changeType))}
              </Box>
            </Box>
          ))}
        </Box>
      ))}
    </Box>
  );
};
