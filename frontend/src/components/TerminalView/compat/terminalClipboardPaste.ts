// 终端「文件/图片 → 路径」粘贴的路径拼装（cc-pane terminalClipboard ②流的 XK 改良）。
//
// 引号权衡（对抗审查确证后的拍板）：
// - 单路径含空白 → 包双引号：claude/正版（imagePaste.ts removeOuterQuotes）剥外层引号后照常
//   识别为图片路径；普通 PowerShell/bash/SSH 远端 shell 则因引号不被空格分词 —— 两边都对。
// - 多路径 → 保持空格裸拼（同 cc-pane formatTerminalFilePaths）：claude 的多路径切分正则按
//   「空格 + 新绝对路径开头」lookahead 切，若逐个包引号则空格后是引号不是盘符/斜杠 → 整串
//   不切分、全部识别失败。含空格的多文件粘进裸 shell 仍会断词，属已知权衡（罕见场景，
//   保 claude 多图附件优先）。
export function formatPastedPaths(paths: string[]): string {
  const first = paths[0];
  if (paths.length === 1 && first !== undefined && /\s/.test(first)) {
    return `"${first}"`;
  }
  return paths.join(' ');
}
