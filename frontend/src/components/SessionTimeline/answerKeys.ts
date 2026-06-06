// AskUserQuestion 作答 → PTY 按键序列。把「卡片点选」翻译成驱动 Claude Code 交互选择器的原始字节。
//
// 为什么是按键：Claude Code 没有「外部注入 tool_result」的 API，运行中的 CLI 唯一输入通道是 PTY
// stdin。所以卡片作答 = 往 PTY 写按键，驱动那个正在等待的 AskUserQuestion 选择器选定（与终端手按等价）。
//
// 选择器真实行为（据 Claude Code 客户端源码确证，非猜测）：多题 = 顶部 tab 条逐题 + 末尾 Submit 复核 tab。
//  - 题间导航：→ = 下一题(tabs:next)、← = 上一题。Select/SelectMulti 不消费左右箭头，故 → 必达 tab 切换。
//  - 单选题(Select)：↑/↓ 移焦点、Enter = 选中并【自动前进到下一 tab】。初始焦点恒在第 0 项；↑ 会回绕，
//    故只用 ↓ 定位（不用「猛按 ↑ 钳顶」—— 那会绕到错项）。单题单选无 Submit tab，Enter 直接提交。
//  - 多选题(SelectMulti，带 Submit 按钮)：Space/Enter = 切换勾选（不提交！），onChange 即记录答案；
//    前进靠 → 到下一 tab（答案已记录，无需走选项末尾的 Submit 按钮）。
//  - 复核 tab(Select{Submit answers / Cancel})：默认焦点在 Submit → Enter 提交全部答案。
//
// ★ 致命时序坑（实测踩中）：选择器的 useInput 回调【闭包捕获当前渲染的 focusedValue / selectedValues】。
//   同一次 PTY 写入（= 同一 stdin data 块）里的多个键，会被 Ink 在一个 tick 内【同步逐个处理】，React
//   来不及在键之间重渲染 → 所有键都读到【同一旧渲染】的陈旧 focus。后果：↓ 移动全部失效、Enter 永远选中
//   第 0 项、Space 全在陈旧 selectedValues 上算成「净选第 0 项」。所以【绝不能把多个键拼一串一次性写入】。
//   解法：buildAnswerKeys 返回【逐个按键】数组，调用方【逐键单独写入、键间留延时】，让 React 在键之间
//   重渲染、刷新闭包，每个键才读到最新 focus。
import { KEY_SEQUENCES } from '@/components/TerminalKeyBar';

const { down: DOWN, enter: ENTER, space: SPACE, right: RIGHT } = KEY_SEQUENCES;

interface AnswerableQuestion {
  multiSelect?: boolean;
  options: unknown[]; // 只取长度（选项数），不关心内容
}

// 构造逐题作答的【逐个按键】序列（每个元素 = 一个键，必须各自单独写入、键间留延时，见上方时序坑）。
// selections: qIdx -> 选中项下标数组（提交前调用方已保证每题≥1）。
export function buildAnswerKeys(
  questions: AnswerableQuestion[],
  selections: Record<number, number[]>,
): string[] {
  const n = questions.length;
  // 单题单选：选择器无 Submit 复核 tab，选中即直接提交（不需要额外的提交键）。
  const hideSubmitTab = n === 1 && !questions[0]?.multiSelect;
  const keys: string[] = [];

  questions.forEach((q, i) => {
    const count = q.options.length;
    const sel = (selections[i] ?? []).filter((x) => x >= 0 && x < count);
    if (q.multiSelect) {
      // 多选：从第 0 项起逐个 ↓ 到目标、Space 切换勾选；末尾 → 推进到下一 tab（答案已由切换记录）。
      const targets = [...new Set(sel)].sort((a, b) => a - b);
      let pos = 0;
      for (const t of targets) {
        for (let d = 0; d < t - pos; d++) keys.push(DOWN);
        keys.push(SPACE);
        pos = t;
      }
      keys.push(RIGHT);
    } else {
      // 单选：从第 0 项 ↓ 到目标、Enter 选中并自动前进（单题单选时此 Enter 即直接提交）。
      const t = sel[0] ?? 0;
      for (let d = 0; d < t; d++) keys.push(DOWN);
      keys.push(ENTER);
    }
  });

  // 多题 / 单题多选：末题推进后停在 Submit 复核 tab，默认焦点「Submit answers」→ Enter 提交。
  if (!hideSubmitTab) keys.push(ENTER);
  return keys;
}
