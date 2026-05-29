import { Fragment, type ReactNode } from 'react';
import { Box, useTheme } from '@mui/material';
import { splitContainerStyle, splitPaneStyle, splitSashSx } from './SplitView.styles';
import { useSplitView } from './useSplitView';

export interface SplitViewProps {
  /** true = 上下分屏，false = 左右分屏。 */
  vertical: boolean;
  /** 各 pane 百分比，如 [50,50]。 */
  sizes: number[];
  /** 每个 pane 最小像素尺寸（默认 50）。 */
  minSize?: number;
  /** 拖拽结束回调（新百分比数组）。 */
  onDragEnd: (sizes: number[]) => void;
  children: ReactNode[];
  /** 与 children 一一对应的稳定 key（避免 2→1 子节点切换时误 remount，复用存活子组件）。 */
  keys: string[];
}

// 自实现 N 路分屏（D6）：flex 布局 + 可拖拽 sash；拖拽时直接改 flexBasis，结束才回调。
export const SplitView = ({ vertical, sizes, minSize, onDragEnd, children, keys }: SplitViewProps) => {
  const theme = useTheme();
  const sashSx = splitSashSx(theme, vertical);
  const { containerRef, effectiveSizes, onSashPointerDown } = useSplitView({
    vertical,
    sizes,
    minSize,
    onDragEnd,
    childCount: children.length,
  });

  return (
    <div ref={containerRef} style={splitContainerStyle(vertical)}>
      {children.map((child, i) => (
        <Fragment key={keys[i] ?? String(i)}>
          {i > 0 && (
            <Box sx={sashSx} onPointerDown={(e) => onSashPointerDown(i - 1, e)} />
          )}
          <div data-splitview-pane style={splitPaneStyle(effectiveSizes[i] ?? 100 / children.length)}>
            {child}
          </div>
        </Fragment>
      ))}
    </div>
  );
};
