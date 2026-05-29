// SplitView 的 ViewModel —— 忠实移植 cc-pane SplitView.tsx 的拖拽机制（D6：自实现分屏）。
//
// 拖拽时直接改 DOM 的 flexBasis（不触发 React re-render，保证拖拽顺滑），pointerup 才
// 回调 onDragEnd 让上层落地新 sizes。尺寸数学走 splitGeometry（已 Vitest 覆盖）。
// 刻意未移植：cc-pane 的全局 setDragging（供 TerminalView drag-fit 节流；本 MVP TerminalView
// 用固定 debounce，无需该信号）。
import { useCallback, useEffect, useRef } from 'react';
import { computeSashResize } from './splitGeometry';

export interface UseSplitViewArgs {
  /** true = 上下分屏（column），false = 左右分屏（row）。 */
  vertical: boolean;
  /** 各 pane 百分比，如 [50,50]。 */
  sizes: number[];
  /** 每个 pane 最小像素尺寸。 */
  minSize?: number;
  /** 拖拽结束回调（新百分比数组，未归一化；上层负责归一化落库）。 */
  onDragEnd: (sizes: number[]) => void;
  /** 子节点数（用于 sizes 长度不匹配时的等分兜底）。 */
  childCount: number;
}

export interface SplitViewModel {
  containerRef: React.RefObject<HTMLDivElement | null>;
  effectiveSizes: number[];
  onSashPointerDown: (index: number, e: React.PointerEvent) => void;
}

export function useSplitView({
  vertical,
  sizes,
  minSize = 50,
  onDragEnd,
  childCount,
}: UseSplitViewArgs): SplitViewModel {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const sizesRef = useRef<number[]>(sizes);
  const cleanupRef = useRef<(() => void) | null>(null);

  // props.sizes → ref（store 更新时保持最新）。
  useEffect(() => {
    sizesRef.current = sizes;
  }, [sizes]);

  // 卸载时清理进行中的拖拽（移除全局监听 + 恢复 body 样式）。
  useEffect(() => () => cleanupRef.current?.(), []);

  const onSashPointerDown = useCallback(
    (index: number, e: React.PointerEvent) => {
      e.preventDefault();
      const container = containerRef.current;
      if (!container) return;

      const panes = container.querySelectorAll<HTMLElement>('[data-splitview-pane]');
      if (panes.length < index + 2) return;

      const totalSize = vertical ? container.clientHeight : container.clientWidth;
      if (totalSize === 0) return;

      const startPos = vertical ? e.clientY : e.clientX;
      const startSizes = [...sizesRef.current];
      const minPercent = (minSize / totalSize) * 100;
      let rafId = 0;

      const onMove = (ev: PointerEvent) => {
        cancelAnimationFrame(rafId);
        rafId = requestAnimationFrame(() => {
          const delta = (((vertical ? ev.clientY : ev.clientX) - startPos) / totalSize) * 100;
          const next = computeSashResize(startSizes, index, delta, minPercent);
          sizesRef.current = next;

          const leftEl = panes[index];
          const rightEl = panes[index + 1];
          const leftSize = next[index];
          const rightSize = next[index + 1];
          if (leftEl && leftSize !== undefined) leftEl.style.flexBasis = `${leftSize}%`;
          if (rightEl && rightSize !== undefined) rightEl.style.flexBasis = `${rightSize}%`;
        });
      };

      const cleanup = () => {
        cancelAnimationFrame(rafId);
        document.removeEventListener('pointermove', onMove);
        document.removeEventListener('pointerup', onUp);
        document.body.style.userSelect = '';
        document.body.style.cursor = '';
        cleanupRef.current = null;
      };

      const onUp = () => {
        cleanup();
        onDragEnd([...sizesRef.current]);
      };

      cleanupRef.current = cleanup;
      document.body.style.userSelect = 'none';
      document.body.style.cursor = vertical ? 'row-resize' : 'col-resize';
      document.addEventListener('pointermove', onMove);
      document.addEventListener('pointerup', onUp);
    },
    [vertical, minSize, onDragEnd],
  );

  const effectiveSizes =
    sizes.length === childCount ? sizes : Array.from({ length: childCount }, () => 100 / childCount);

  return { containerRef, effectiveSizes, onSashPointerDown };
}
