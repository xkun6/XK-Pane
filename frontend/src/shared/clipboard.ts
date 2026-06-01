// 复制文本到剪贴板，带非安全上下文兜底。
//
// `navigator.clipboard` 是**安全上下文限定 API**：P7 的 Web 访问走「LAN IP + 明文 HTTP」
// （如 http://192.168.x.x:9317）属非安全上下文 → `navigator.clipboard` 为 undefined，
// 直接 `navigator.clipboard?.writeText` 会被可选链静默吞掉（手机上「复制」点了没反应）。
// 故先试 Clipboard API，失败再回退到临时 textarea + execCommand('copy')（非安全上下文也可用）。
// 返回是否成功，调用方据此给 UI 反馈（Snackbar）。
export async function copyText(text: string): Promise<boolean> {
  try {
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // 安全上下文下也可能被权限策略拒绝 → 落到 execCommand 兜底。
  }
  if (typeof document === 'undefined') return false;
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.top = '-1000px';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}
