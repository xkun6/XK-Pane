// toolMeta：把工具名映射到「展示元数据」—— 图标 + 色调 + 一行摘要。让每种工具卡片有可辨识的身份
// （对标 claude-code-webui 的 per-tool 渲染，但用 @mui/icons-material 的 *Rounded 图标 + theme 语义色，
// 不照搬它的 emoji + Tailwind 硬编码色）。纯展示辅助，无状态。
import type { SvgIconComponent } from '@mui/icons-material';
import TerminalRounded from '@mui/icons-material/TerminalRounded';
import MenuBookRounded from '@mui/icons-material/MenuBookRounded';
import NoteAddRounded from '@mui/icons-material/NoteAddRounded';
import DriveFileRenameOutlineRounded from '@mui/icons-material/DriveFileRenameOutlineRounded';
import EditNoteRounded from '@mui/icons-material/EditNoteRounded';
import FolderOpenRounded from '@mui/icons-material/FolderOpenRounded';
import SearchRounded from '@mui/icons-material/SearchRounded';
import LanguageRounded from '@mui/icons-material/LanguageRounded';
import TravelExploreRounded from '@mui/icons-material/TravelExploreRounded';
import SmartToyRounded from '@mui/icons-material/SmartToyRounded';
import ChecklistRounded from '@mui/icons-material/ChecklistRounded';
import FactCheckRounded from '@mui/icons-material/FactCheckRounded';
import ExtensionRounded from '@mui/icons-material/ExtensionRounded';
import BuildRounded from '@mui/icons-material/BuildRounded';

// 色调（解析为 theme.palette.foundation.* 的语义色，见 SessionTimeline.styles 的 resolveTone）。
// 只读类工具（Read/Grep/Glob/Web*）用 muted 低调，会改东西/执行类用 accent 强调。
export type ToolTone = 'accent' | 'muted';

export interface ToolMeta {
  Icon: SvgIconComponent;
  tone: ToolTone;
}

// 只读 / 不改文件的工具 → 低调灰，视觉上让「会改东西」的工具更突出。
const READ_ONLY = new Set(['Read', 'Grep', 'Glob', 'WebFetch', 'WebSearch', 'NotebookRead', 'LS']);

const ICONS: Record<string, SvgIconComponent> = {
  Bash: TerminalRounded,
  BashOutput: TerminalRounded,
  Read: MenuBookRounded,
  Write: NoteAddRounded,
  Edit: DriveFileRenameOutlineRounded,
  MultiEdit: DriveFileRenameOutlineRounded,
  NotebookEdit: EditNoteRounded,
  Glob: FolderOpenRounded,
  Grep: SearchRounded,
  WebFetch: LanguageRounded,
  WebSearch: TravelExploreRounded,
  Task: SmartToyRounded,
  TodoWrite: ChecklistRounded,
  ExitPlanMode: FactCheckRounded,
};

// getToolMeta：按工具名取图标 + 色调。未知 mcp__* 工具用「扩展」图标，其余未知用通用「工具」图标。
export function getToolMeta(toolName?: string): ToolMeta {
  const name = toolName ?? '';
  let Icon = ICONS[name];
  if (!Icon && name.startsWith('mcp__')) Icon = ExtensionRounded;
  if (!Icon) Icon = BuildRounded;
  return { Icon, tone: READ_ONLY.has(name) ? 'muted' : 'accent' };
}

// parseToolInput：把后端的紧凑 JSON 字符串解析成对象（失败返回 null，调用方回退原串）。
export function parseToolInput(raw?: string): Record<string, unknown> | null {
  if (!raw) return null;
  try {
    const o = JSON.parse(raw) as unknown;
    return o && typeof o === 'object' ? (o as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

// summarizeToolInput：按工具名挑最有信息量的字段，给折叠卡 header 一行预览。
export function summarizeToolInput(
  toolName: string | undefined,
  input: Record<string, unknown> | null,
): string {
  if (!input) return '';
  const str = (k: string): string => (typeof input[k] === 'string' ? (input[k] as string) : '');
  switch (toolName) {
    case 'Bash':
      return str('command');
    case 'Read':
    case 'Write':
    case 'Edit':
    case 'MultiEdit':
    case 'NotebookEdit':
      return str('file_path') || str('path') || str('notebook_path');
    case 'Glob': {
      const p = str('path');
      return p ? `${str('pattern')}  ${p}` : str('pattern');
    }
    case 'Grep':
      return str('pattern');
    case 'WebFetch':
      return str('url');
    case 'WebSearch':
      return str('query');
    case 'Task':
      return str('description') || str('subagent_type');
    default:
      return (
        str('command') ||
        str('file_path') ||
        str('path') ||
        str('pattern') ||
        str('url') ||
        str('description') ||
        str('query') ||
        str('prompt')
      );
  }
}

// basename：取路径最后一段（用于 diff 卡片 header 简短显示文件名，正反斜杠都切）。
export function basename(p?: string): string {
  if (!p) return '';
  const parts = p.split(/[\\/]/);
  return parts[parts.length - 1] || p;
}
