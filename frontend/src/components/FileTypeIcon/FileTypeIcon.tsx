import type { SvgIconProps } from '@mui/material';
import InsertDriveFileRoundedIcon from '@mui/icons-material/InsertDriveFileRounded';
import ImageRoundedIcon from '@mui/icons-material/ImageRounded';
import MovieRoundedIcon from '@mui/icons-material/MovieRounded';
import MusicNoteRoundedIcon from '@mui/icons-material/MusicNoteRounded';
import PictureAsPdfRoundedIcon from '@mui/icons-material/PictureAsPdfRounded';
import FolderZipRoundedIcon from '@mui/icons-material/FolderZipRounded';
import JavascriptRoundedIcon from '@mui/icons-material/JavascriptRounded';
import HtmlRoundedIcon from '@mui/icons-material/HtmlRounded';
import CssRoundedIcon from '@mui/icons-material/CssRounded';
import DataObjectRoundedIcon from '@mui/icons-material/DataObjectRounded';
import ArticleRoundedIcon from '@mui/icons-material/ArticleRounded';
import TableChartRoundedIcon from '@mui/icons-material/TableChartRounded';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import CodeRoundedIcon from '@mui/icons-material/CodeRounded';

// 文件类型图标：按扩展名渲染高辨识度的 @mui *Rounded 图标 + 类型色。
//
// 配色说明（foundation 配色铁律的明确例外）：文件类型色刻意硬编码为「品牌中间调」，
// 目的是跨明/暗/纯黑三套主题都保持稳定可辨（红 PDF / 黄 JS / 蓝 CSS …）。这些中间调
// 在三种背景下对比都达标，且语义是「类型身份」而非「主题装饰」，故不从 theme.palette 取。

type IconComp = typeof InsertDriveFileRoundedIcon;
interface Spec {
  Icon: IconComp;
  color: string;
}

const C = {
  red: '#e5534b',
  orange: '#d18616',
  yellow: '#d4a72c',
  green: '#3fa45b',
  teal: '#1aa6a0',
  blue: '#4c8edb',
  indigo: '#6e7bf2',
  purple: '#a371f7',
  pink: '#d2618b',
  brown: '#a3743f',
  gray: '#8b949e',
} as const;

const DEFAULT_SPEC: Spec = { Icon: InsertDriveFileRoundedIcon, color: C.gray };

// 扩展名 → 图标规格。未命中走 DEFAULT_SPEC。
const EXT_SPEC: Record<string, Spec> = {
  // 图片
  ...mapExts(['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'avif'], ImageRoundedIcon, C.teal),
  // 视频
  ...mapExts(['mp4', 'mov', 'avi', 'mkv', 'webm', 'm4v', 'flv'], MovieRoundedIcon, C.purple),
  // 音频
  ...mapExts(['mp3', 'wav', 'flac', 'ogg', 'm4a', 'aac'], MusicNoteRoundedIcon, C.pink),
  // PDF
  ...mapExts(['pdf'], PictureAsPdfRoundedIcon, C.red),
  // 压缩包
  ...mapExts(['zip', 'rar', '7z', 'tar', 'gz', 'bz2', 'xz', 'tgz'], FolderZipRoundedIcon, C.brown),
  // JS 家族
  ...mapExts(['js', 'jsx', 'cjs', 'mjs'], JavascriptRoundedIcon, C.yellow),
  // TS 家族（无专属 TS 图标，用 Code + 蓝色区分）
  ...mapExts(['ts', 'tsx', 'mts', 'cts'], CodeRoundedIcon, C.blue),
  // HTML
  ...mapExts(['html', 'htm', 'vue', 'svelte'], HtmlRoundedIcon, C.orange),
  // CSS
  ...mapExts(['css', 'scss', 'sass', 'less'], CssRoundedIcon, C.blue),
  // 结构化数据 / 配置
  ...mapExts(['json', 'yaml', 'yml', 'toml', 'jsonc'], DataObjectRoundedIcon, C.orange),
  ...mapExts(['ini', 'conf', 'cfg', 'env', 'properties'], DataObjectRoundedIcon, C.gray),
  // Markdown
  ...mapExts(['md', 'markdown', 'mdx'], ArticleRoundedIcon, C.blue),
  // 表格
  ...mapExts(['csv', 'tsv', 'xlsx', 'xls'], TableChartRoundedIcon, C.green),
  // 脚本 / shell
  ...mapExts(['sh', 'bash', 'zsh', 'fish', 'bat', 'cmd', 'ps1'], TerminalRoundedIcon, C.green),
  // 纯文本 / 日志
  ...mapExts(['txt', 'log', 'rtf'], DescriptionRoundedIcon, C.gray),
  // 各语言源码（统一 Code 图标，配色区分大类）
  ...mapExts(['go'], CodeRoundedIcon, C.teal),
  ...mapExts(['py', 'pyw'], CodeRoundedIcon, C.blue),
  ...mapExts(['rs'], CodeRoundedIcon, C.brown),
  ...mapExts(['rb'], CodeRoundedIcon, C.red),
  ...mapExts(['php'], CodeRoundedIcon, C.indigo),
  ...mapExts(['java', 'kt', 'kts'], CodeRoundedIcon, C.orange),
  ...mapExts(['c', 'h', 'cpp', 'cc', 'hpp', 'cs', 'm', 'mm'], CodeRoundedIcon, C.indigo),
  ...mapExts(['swift', 'dart', 'lua', 'scala', 'pl', 'r'], CodeRoundedIcon, C.indigo),
};

function mapExts(exts: string[], Icon: IconComp, color: string): Record<string, Spec> {
  const out: Record<string, Spec> = {};
  for (const e of exts) out[e] = { Icon, color };
  return out;
}

export interface FileTypeIconProps {
  name: string;
  fontSize?: SvgIconProps['fontSize'];
}

// 按文件名扩展名渲染类型化彩色图标（仅文件；目录图标由调用方按 accent 渲染）。
export const FileTypeIcon = ({ name, fontSize = 'small' }: FileTypeIconProps) => {
  const dot = name.lastIndexOf('.');
  const ext = dot > 0 ? name.slice(dot + 1).toLowerCase() : '';
  const spec = EXT_SPEC[ext] ?? DEFAULT_SPEC;
  const Icon = spec.Icon;
  return <Icon fontSize={fontSize} sx={{ color: spec.color }} />;
};
