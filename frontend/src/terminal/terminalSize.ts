// 终端「最近一次成功 fit 的真实列宽/行高」模块级快照。
//
// 用途：新建 / resume 会话时给后端 PTY 一个贴近真实显示尺寸的初始列宽。否则 PTY 用固定默认
// （ptyx 80 / orchestrator 120），而 reclaude --resume 一启动就按该默认列宽把历史对话折行重放进
// 终端 —— 与前端 xterm 实际 fit 出的列宽不符时整屏错位（换 JetBrains Mono 把 cell 变宽后差距更大）。
// 事后 attach 的 resize 来晚了：reclaude 已重放完、scrollback 折行已定，且列宽不再变化就不触发
// xterm reflow，只有手动最大化/最小化两次逼出 reflow 才恢复。把创建时的初始列宽设成「当前面板真实
// 列宽」，reclaude 从一开始就按对的列宽重放，根除错位。
//
// 同一终端面板的新 Tab 与已有 Tab 同宽，故「最近 fit 值」对 resume 场景足够准；仍有偏差时 attach
// 的 force fit 会再微调。桌面单页应用，模块级单例所有终端面板共享（与 layoutScheduler 闭包 lastSize
// 同源思路）。cols=0 表示尚无任何终端 fit 过（冷启动首个会话），此时回退后端默认。
let cols = 0;
let rows = 0;

export function recordTerminalSize(c: number, r: number): void {
  if (c > 0 && r > 0) {
    cols = c;
    rows = r;
  }
}

export function lastTerminalSize(): { cols: number; rows: number } {
  return { cols, rows };
}
