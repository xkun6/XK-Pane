import type { TimelineEvent, TimelineQuestion } from '@/services';

// derivePending：从已累积的转录事件派生「当前待答的 AskUserQuestion」——事件漏掉时的可靠回退。
// 命门 —— ask-pending 是 fire-and-forget、无重放的一次性事件：web viewer 事后挂载 / /ws 重连会永久
// 错过它 → pending 恒空、卡片退化成只读点不动（桌面 owner 全程在场不受影响）。但待答期间该问题自己的
// assistant tool_use 行已落盘转录（仅缺 tool_result），轮询即可读到 → 据此派生 pending 兜底；答完
// (tool_result 落地)后该 tool_use 即被视为已答、自动消解。返回最后一个未作答 AskUserQuestion 的问题集。
// 「未作答」判定与 useSessionTimeline 的 answeredAskCount 一致：有 toolUseId 且无匹配 tool_result
// （无 toolUseId 无从追踪 → 当已答，避免与转录只读卡重复渲染）。
export function derivePending(events: TimelineEvent[]): TimelineQuestion[] | null {
  const resultIds = new Set<string>();
  for (const ev of events) if (ev.kind === 'tool_result' && ev.toolUseId) resultIds.add(ev.toolUseId);
  let found: TimelineQuestion[] | null = null;
  for (const ev of events) {
    if (
      ev.kind === 'tool_use' &&
      ev.toolName === 'AskUserQuestion' &&
      ev.toolUseId &&
      !resultIds.has(ev.toolUseId) &&
      ev.questions &&
      ev.questions.length > 0
    ) {
      found = ev.questions; // 取最后一个未作答的（claude 逐个阻塞作答，正常仅一个在场）
    }
  }
  return found;
}
