// AskUserQuestion 作答 → PTY 按键序列。把「卡片点选」翻译成驱动 Claude Code 交互选择器的原始字节。
//
// 为什么是按键：Claude Code 没有「外部注入 tool_result」的 API，运行中的 CLI 唯一输入通道是 PTY
// stdin。所以卡片作答 = 往 PTY 写按键，驱动那个正在等待的交互选择器选定某项（与终端里手按方向键等价）。
//
// 关键假设（Claude Code 选择器 TUI 的行为；若实测按键不对，只改这一处即可校准）：
//  - 选项纵向排列、↑/↓ 移动高亮、到顶/底**钳制**不回绕 → 故「猛按 ↑ 到顶」可定位绝对起点（index 0），
//    使序列与「当前高亮在哪」无关（健壮，不依赖选择器初始光标位置）。
//  - 选项列表下方有一个隐式「Other（自定义）」自由文本项，计入可导航行数（故 ↑ 多按一次兜底）。
//  - 单选：Enter 确认当前高亮。多选：Space 切换勾选，Enter 提交。
//
// 多问题（一次 AskUserQuestion 含多题）：选择器逐题作答，每题 Enter 后前进到下一题且高亮重置到顶。
// 卡片是「表单式」一次性提交：buildAllAnswersKeys 按题序拼接各题序列成一条完整按键串，于「提交」时整体写入。
import { KEY_SEQUENCES } from '@/components/TerminalKeyBar';

const { up: UP, down: DOWN, enter: ENTER, space: SPACE } = KEY_SEQUENCES;

// clampToTop：足量 ↑ 把高亮钳到顶部 index 0。optionCount + 1 覆盖底部的隐式「Other」行；
// 钳制语义下多按的 ↑ 是 no-op，故宁多勿少。
function clampToTop(optionCount: number): string {
  return UP.repeat(Math.max(1, optionCount + 1));
}

// 单选：选中第 optionIndex 个选项（0 基）。optionCount = 该题列出的选项数（不含 Other）。
// 序列：↑×(n+1) 钳到顶 → ↓×optionIndex 到目标 → Enter 确认。
export function buildSingleSelectKeys(optionIndex: number, optionCount: number): string {
  const target = Math.max(0, optionIndex);
  return clampToTop(optionCount) + DOWN.repeat(target) + ENTER;
}

// 多选：勾选 indices 指定的若干选项后提交。
// 序列：↑×(n+1) 钳到顶 → 对每个目标(升序) ↓ 到该项 + Space 切换 → 末尾 Enter 提交。
export function buildMultiSelectKeys(indices: number[], optionCount: number): string {
  const sorted = [...new Set(indices)].filter((i) => i >= 0 && i < optionCount).sort((a, b) => a - b);
  let keys = clampToTop(optionCount);
  let pos = 0;
  for (const idx of sorted) {
    keys += DOWN.repeat(idx - pos) + SPACE;
    pos = idx;
  }
  return keys + ENTER;
}

// 表单式批量作答：把整张卡所有问题的选择，按题序拼成一条完整按键序列，于「提交」时一次性写入 PTY。
// 选择器逐题推进：每题末尾 Enter 确认并前进到下一题（高亮重置到顶），故顺次拼接各题序列即可驱动到底，
// 末题的 Enter 提交整个 AskUserQuestion。selections: qIdx -> 选中项下标数组（提交前调用方已保证每题≥1）。
interface AnswerableQuestion {
  multiSelect?: boolean;
  options: unknown[]; // 只取长度（选项数），不关心内容 → 保持本模块为纯字节构造器、不依赖 services 类型
}

export function buildAllAnswersKeys(
  questions: AnswerableQuestion[],
  selections: Record<number, number[]>,
): string {
  let keys = '';
  questions.forEach((q, qIdx) => {
    const sel = selections[qIdx] ?? [];
    const count = q.options.length;
    keys += q.multiSelect ? buildMultiSelectKeys(sel, count) : buildSingleSelectKeys(sel[0] ?? 0, count);
  });
  return keys;
}
