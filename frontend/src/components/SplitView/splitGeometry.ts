// SplitView 的纯尺寸几何 —— 从 cc-pane SplitView.tsx 拖拽逻辑 + SplitContainer.handleDragEnd
// 抽出的无副作用函数，便于 Vitest 覆盖（拖拽 DOM 交互不易测，但尺寸数学是出 bug 的地方）。

/**
 * 拖拽第 index 个 sash（介于 pane[index] 与 pane[index+1] 之间）时，按 deltaPercent 重算这两个
 * pane 的百分比，并施加最小百分比约束（min 顶到边时把溢出转嫁给相邻 pane）。其余 pane 不变。
 * 忠实复刻 cc-pane SplitView 的 newLeft/newRight 约束算法。
 */
export function computeSashResize(
  sizes: number[],
  index: number,
  deltaPercent: number,
  minPercent: number,
): number[] {
  const startLeft = sizes[index] ?? 0;
  const startRight = sizes[index + 1] ?? 0;

  let newLeft = startLeft + deltaPercent;
  let newRight = startRight - deltaPercent;

  if (newLeft < minPercent) {
    newRight += newLeft - minPercent;
    newLeft = minPercent;
  }
  if (newRight < minPercent) {
    newLeft += newRight - minPercent;
    newRight = minPercent;
  }

  const next = [...sizes];
  next[index] = newLeft;
  next[index + 1] = newRight;
  return next;
}

/**
 * 归一化为「和恰为 100」的百分比数组（末位吸收舍入误差）。
 * 忠实复刻 cc-pane SplitContainer.handleDragEnd。total<=0 / 空数组原样返回。
 */
export function normalizeSizesToHundred(sizes: number[]): number[] {
  const total = sizes.reduce((a, b) => a + b, 0);
  if (total <= 0 || sizes.length === 0) return sizes;

  const rounded = sizes.map((s) => Math.round((s / total) * 1000) / 10);
  const sumExceptLast = rounded.slice(0, -1).reduce((a, b) => a + b, 0);
  rounded[rounded.length - 1] = Math.round((100 - sumExceptLast) * 10) / 10;
  return rounded;
}
