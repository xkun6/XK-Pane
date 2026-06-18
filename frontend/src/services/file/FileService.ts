// FileService：文件浏览器后端能力的前端门面，包 @bindings/xkpane file service。
//
// 后端是桌面专属能力（直接读写本机文件系统）。Web（viewer）端 REST 白名单不暴露任何
// 文件端点，故这里对所有方法做 isDesktop 守卫：Web 端调用主动抛错（让上层 UI 降级提示），
// 绝不静默失败（保存丢失风险）。桌面端经类型安全的 Wails binding 调用。
import { Service as Binding } from '@bindings/xkpane/internal/services/file';
import type {
  FileEntry as RawEntry,
  FileContent as RawContent,
  ContentMatch as RawMatch,
} from '@bindings/xkpane/internal/services/file/models';
import type { RecentDirectory as RawRecent } from '@bindings/xkpane/internal/storage/models';
import { isDesktop } from '@/services/transport';

// 目录条目（os.DirEntry 的展示 DTO）。
export interface FileEntry {
  name: string;
  path: string; // 原样拼接的绝对路径
  isDir: boolean;
  size: number; // 字节；目录为 0
  modTime: number; // unix 毫秒
}

// 文件内容 + 元信息。isBinary=true 时 content 为空（二进制不进文本编辑器）。
export interface FileContent {
  path: string;
  content: string;
  size: number;
  isBinary: boolean;
}

// 最近打开的目录（文件浏览器入口快捷区）。
export interface RecentDirectory {
  id: number;
  path: string;
  openCount: number;
  lastOpenedAt: string; // ISO 时间串
}

// 一条全文内容命中（文件 + 行号 + 行预览）。
export interface ContentMatch {
  path: string; // 绝对路径
  rel: string; // 相对 root 的展示路径
  line: number; // 1-based 行号
  text: string; // 命中行内容
}

// Web 端误用守卫：抛错而非静默失败。
function assertDesktop(): void {
  if (!isDesktop) {
    throw new Error('file: 文件浏览器仅在桌面端可用（Web 端无文件系统访问）');
  }
}

function toEntry(e: RawEntry): FileEntry {
  return {
    name: e.name ?? '',
    path: e.path ?? '',
    isDir: e.isDir ?? false,
    size: e.size ?? 0,
    modTime: e.modTime ?? 0,
  };
}

function toContent(c: RawContent): FileContent {
  return {
    path: c.path ?? '',
    content: c.content ?? '',
    size: c.size ?? 0,
    isBinary: c.isBinary ?? false,
  };
}

function toRecent(r: RawRecent): RecentDirectory {
  return {
    id: r.id ?? 0,
    path: r.path ?? '',
    openCount: r.openCount ?? 0,
    lastOpenedAt: typeof r.lastOpenedAt === 'string' ? r.lastOpenedAt : '',
  };
}

export const FileService = {
  // 列目录直接子项（目录在前、文件在后，名称大小写不敏感升序）。
  async listDir(path: string): Promise<FileEntry[]> {
    assertDesktop();
    const rows = await Binding.ListDir(path);
    return (rows ?? []).map(toEntry);
  },

  // 读文件文本内容（二进制返 isBinary=true 且 content 空）。
  async readFile(path: string): Promise<FileContent> {
    assertDesktop();
    return toContent(await Binding.ReadFile(path));
  },

  // 原子写回文件。
  async writeFile(path: string, content: string): Promise<void> {
    assertDesktop();
    await Binding.WriteFile(path, content);
  },

  // 在系统文件管理器中定位（目录打开 / 文件选中）。
  async revealInOS(path: string): Promise<void> {
    assertDesktop();
    await Binding.RevealInOS(path);
  },

  // 登记一个「文件浏览器打开的根目录」（去重 upsert）。
  async recordRecent(path: string): Promise<void> {
    assertDesktop();
    await Binding.RecordRecent(path);
  },

  // 最近打开的目录（按最后打开时间倒序）。
  async listRecent(limit = 20): Promise<RecentDirectory[]> {
    assertDesktop();
    const rows = await Binding.ListRecent(limit);
    return (rows ?? []).map(toRecent);
  },

  // 读文件为 data URI（图片等非文本就地预览；超 16MB 后端报错）。
  async readFileBase64(path: string): Promise<string> {
    assertDesktop();
    return Binding.ReadFileBase64(path);
  },

  // 在 dir 下新建空文件；返回新文件绝对路径（已存在则报错）。
  async createFile(dir: string, name: string): Promise<string> {
    assertDesktop();
    return Binding.CreateFile(dir, name);
  },

  // 在 dir 下新建子目录；返回新目录绝对路径（已存在则报错）。
  async createDir(dir: string, name: string): Promise<string> {
    assertDesktop();
    return Binding.CreateDir(dir, name);
  },

  // 重命名为同目录下的 newName；返回新路径。
  async rename(path: string, newName: string): Promise<string> {
    assertDesktop();
    return Binding.Rename(path, newName);
  },

  // 移动到目标目录（同名自动加序号；移回原目录为无操作）；返回最终路径。
  async move(src: string, dstDir: string): Promise<string> {
    assertDesktop();
    return Binding.Move(src, dstDir);
  },

  // 复制到目标目录（源保留，同名自动加序号）；返回副本路径。
  async copy(src: string, dstDir: string): Promise<string> {
    assertDesktop();
    return Binding.Copy(src, dstDir);
  },

  // 在同目录复制副本（名字加序号）；返回副本路径。
  async duplicate(path: string): Promise<string> {
    assertDesktop();
    return Binding.Duplicate(path);
  },

  // 移到系统回收站（可恢复）。
  async trash(path: string): Promise<void> {
    assertDesktop();
    await Binding.Trash(path);
  },

  // 全文内容搜索（grep 式，不区分大小写；三重预算限制，跳过忽略目录与二进制）。
  async searchContent(root: string, query: string): Promise<ContentMatch[]> {
    assertDesktop();
    const rows = await Binding.SearchContent(root, query);
    return (rows ?? []).map((m: RawMatch) => ({
      path: m.path ?? '',
      rel: m.rel ?? '',
      line: m.line ?? 0,
      text: m.text ?? '',
    }));
  },
} as const;
