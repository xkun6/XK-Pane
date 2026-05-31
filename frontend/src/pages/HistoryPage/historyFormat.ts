// HistoryPage 的纯展示格式化工具（非 UI 文案，不走 i18n）：
//   - formatTime：把 RFC3339 时间戳按运行时本地习惯格式化（toLocaleString，无硬编码语言串）
//   - formatSize：把字节数格式化为 B/KB/MB/GB（单位为通用符号，无需翻译）

export function formatTime(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

export function formatSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '0 B';
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = bytes / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i += 1;
  }
  return `${value.toFixed(value >= 100 || Number.isInteger(value) ? 0 : 1)} ${units[i]}`;
}
