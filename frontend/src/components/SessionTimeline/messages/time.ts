// formatTime：把 ISO 时间戳格式化为本地 HH:mm:ss（卡片角落小灰字）。无效 / 空返回空串。
export function formatTime(ts?: string): string {
  if (!ts) return '';
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}
