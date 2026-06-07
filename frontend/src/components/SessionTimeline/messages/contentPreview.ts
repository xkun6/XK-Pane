// contentPreview：把长文本截成「首 N 行预览」，让折叠态也能瞥见内容（对标 claude-code-webui 的
// createContentPreview —— 折叠卡不再只剩一行标题，而是露出前几行 + 「还有 N 行」提示）。纯函数。
export interface ContentPreview {
  preview: string; // 前 maxLines 行（或全文，若不足）
  hasMore: boolean; // 是否被截断（总行数 > maxLines）
  totalLines: number;
  previewLines: number;
}

export function createContentPreview(content: string, maxLines = 6): ContentPreview {
  if (!content || content.trim().length === 0) {
    return { preview: '', hasMore: false, totalLines: 0, previewLines: 0 };
  }
  const lines = content.split('\n');
  const totalLines = lines.length;
  if (totalLines <= maxLines) {
    return { preview: content, hasMore: false, totalLines, previewLines: totalLines };
  }
  return {
    preview: lines.slice(0, maxLines).join('\n'),
    hasMore: true,
    totalLines,
    previewLines: maxLines,
  };
}
