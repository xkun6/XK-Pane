// AskUserQuestion 作答 → PTY 按键序列。把「卡片点选」翻译成驱动 Claude Code 交互选择器的原始字节。
//
// 为什么是按键：Claude Code 没有「外部注入 tool_result」的 API，运行中的 CLI 唯一输入通道是 PTY
// stdin。所以卡片作答 = 往 PTY 写按键，驱动那个正在等待的 AskUserQuestion 选择器选定（与终端手按等价）。
//
// 选择器真实行为（据 Claude Code 客户端源码确证，非猜测）：多题 = 顶部 tab 条逐题 + 末尾 Submit 复核 tab。
//  - 题间导航：→ = 下一题(tabs:next)、← = 上一题。Select/SelectMulti 不消费左右箭头，故 → 必达 tab 切换。
//  - 单选题(Select)：↑/↓ 移焦点、Enter = 选中并【自动前进到下一 tab】。初始焦点恒在第 0 项；↑ 会回绕，
//    故只用 ↓ 定位（不用「猛按 ↑ 钳顶」—— 那会绕圈到错项）。单题单选无 Submit tab，Enter 直接提交。
//  - 多选题(SelectMulti，带 Submit 按钮)：Space/Enter = 切换勾选（不提交！），onChange 即记录答案；
//    前进靠 → 到下一 tab（答案已记录，无需走选项末尾的 Submit 按钮）。
//  - 复核 tab(Select{Submit answers / Cancel})：默认焦点在 Submit → Enter 提交全部答案。
//
// 时序坑：每题 keyed 独立挂载、初始焦点第 0 项。一次性猛灌按键会在切 tab 瞬间丢键，故【分段】返回 ——
// 每题一段（含末尾推进），多题再追加一段提交 Enter；调用方逐段写入、段间留小延时（让新 tab 挂载后再收键）。
import { KEY_SEQUENCES } from '@/components/TerminalKeyBar';

const { down: DOWN, enter: ENTER, space: SPACE, right: RIGHT } = KEY_SEQUENCES;

interface AnswerableQuestion {
  multiSelect?: boolean;
  options: unknown[]; // 只取长度（选项数），不关心内容
}

// 构造逐题作答的按键「分段」序列。selections: qIdx -> 选中项下标数组（提交前调用方已保证每题≥1）。
export function buildAnswerChunks(
  questions: AnswerableQuestion[],
  selections: Record<number, number[]>,
): string[] {
  const n = questions.length;
  // 单题单选：选择器无 Submit 复核 tab，选中即直接提交（不需要额外的提交段）。
  const hideSubmitTab = n === 1 && !questions[0]?.multiSelect;
  const chunks: string[] = [];

  questions.forEach((q, i) => {
    const count = q.options.length;
    const sel = (selections[i] ?? []).filter((x) => x >= 0 && x < count);
    if (q.multiSelect) {
      // 多选：从第 0 项起逐个 ↓ 到目标 + Space 切换勾选；末尾 → 推进到下一 tab（答案已由切换记录）。
      const targets = [...new Set(sel)].sort((a, b) => a - b);
      let pos = 0;
      let keys = '';
      for (const t of targets) {
        keys += DOWN.repeat(t - pos) + SPACE;
        pos = t;
      }
      chunks.push(keys + RIGHT);
    } else {
      // 单选：从第 0 项 ↓ 到目标 + Enter（选中并自动前进；单题单选时此 Enter 即直接提交）。
      const t = sel[0] ?? 0;
      chunks.push(DOWN.repeat(t) + ENTER);
    }
  });

  // 多题 / 单题多选：末题推进后停在 Submit 复核 tab，默认焦点「Submit answers」→ Enter 提交。
  if (!hideSubmitTab) chunks.push(ENTER);
  return chunks;
}
