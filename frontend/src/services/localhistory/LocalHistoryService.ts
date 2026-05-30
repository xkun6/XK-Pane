// LocalHistoryService：本地历史（文件版本快照）的前端封装，包 @bindings/xkpane localhistorysvc。
//
// 后端是每项目独立 db（D10），service 按 projectPath 操作。本封装当前收口历史面板第一阶段
// 所需子集：recentChanges（跨文件最近变更）/ snapshotFile（手动快照）/ restoreVersion（回滚）。
// diff 查看（getVersionDiff / getVersionsDiff）+ 版本列表 + 标签管理留后续单元（diff 渲染较重）。
import { Service as Binding } from '@bindings/xkpane/internal/services/localhistorysvc';
import type {
  RecentChange as RawRecent,
  FileVersion as RawVersion,
  DiffResult as RawDiff,
  DiffHunk as RawHunk,
  DiffLine as RawLine,
  InlineChange as RawInline,
} from '@bindings/xkpane/internal/localhistory/models';

// 一条最近变更（跨文件）。labelName 空 = 该版本无关联标签。
export interface RecentChange {
  filePath: string;
  versionId: string;
  timestamp: string; // RFC3339
  size: number;
  hash: string;
  labelName: string;
  branch: string;
}

// 一条文件版本元数据。
export interface FileVersion {
  id: string;
  filePath: string;
  hash: string;
  size: number;
  createdAt: string;
  isDeleted: boolean;
  branch: string;
}

// —— diff DTO（对齐后端 localhistory.DiffResult；changeType snake_case 枚举）——
export type DiffChangeType = 'equal' | 'insert' | 'delete' | 'replace';

// 行内字符级变更标记。start/end 为 **rune（码点）下标**（非 UTF-16 单元）——渲染须按码点切片。
export interface InlineChange {
  start: number;
  end: number;
  changeType: DiffChangeType;
}

export interface DiffLine {
  changeType: DiffChangeType;
  content: string;
  oldLineNo: number | null;
  newLineNo: number | null;
  inlineChanges?: InlineChange[];
}

export interface DiffHunk {
  oldStart: number;
  oldCount: number;
  newStart: number;
  newCount: number;
  lines: DiffLine[];
}

export interface DiffStats {
  additions: number;
  deletions: number;
  changes: number;
}

export interface DiffResult {
  hunks: DiffHunk[];
  stats: DiffStats;
  isBinary: boolean;
  truncated: boolean;
}

function toInline(c: RawInline): InlineChange {
  return {
    start: c.start ?? 0,
    end: c.end ?? 0,
    changeType: (c.changeType ?? 'equal') as DiffChangeType,
  };
}

function toLine(l: RawLine): DiffLine {
  const inlines = l.inlineChanges?.map(toInline);
  return {
    changeType: (l.changeType ?? 'equal') as DiffChangeType,
    content: l.content ?? '',
    oldLineNo: typeof l.oldLineNo === 'number' ? l.oldLineNo : null,
    newLineNo: typeof l.newLineNo === 'number' ? l.newLineNo : null,
    ...(inlines && inlines.length > 0 ? { inlineChanges: inlines } : {}),
  };
}

function toHunk(h: RawHunk): DiffHunk {
  return {
    oldStart: h.oldStart ?? 0,
    oldCount: h.oldCount ?? 0,
    newStart: h.newStart ?? 0,
    newCount: h.newCount ?? 0,
    lines: (h.lines ?? []).map(toLine),
  };
}

function toDiff(d: RawDiff): DiffResult {
  return {
    hunks: (d.hunks ?? []).map(toHunk),
    stats: {
      additions: d.stats?.additions ?? 0,
      deletions: d.stats?.deletions ?? 0,
      changes: d.stats?.changes ?? 0,
    },
    isBinary: d.isBinary ?? false,
    truncated: d.truncated ?? false,
  };
}

function toRecent(r: RawRecent): RecentChange {
  return {
    filePath: r.filePath ?? '',
    versionId: r.versionId ?? '',
    timestamp: r.timestamp ?? '',
    size: r.size ?? 0,
    hash: r.hash ?? '',
    labelName: r.labelName ?? '',
    branch: r.branch ?? '',
  };
}

function toVersion(v: RawVersion): FileVersion {
  return {
    id: v.id ?? '',
    filePath: v.filePath ?? '',
    hash: v.hash ?? '',
    size: v.size ?? 0,
    createdAt: v.createdAt ?? '',
    isDeleted: v.isDeleted ?? false,
    branch: v.branch ?? '',
  };
}

export const LocalHistoryService = {
  // 最近版本变更（跨文件，最新在前）；limit<=0 后端取默认 50。
  async recentChanges(projectPath: string, limit: number): Promise<RecentChange[]> {
    const rows = await Binding.GetRecentChanges(projectPath, limit);
    return (rows ?? []).map(toRecent);
  },

  // 读工作树文件并存一个版本（相对路径）。内容未变因去重返回 null。
  async snapshotFile(projectPath: string, filePath: string, branch: string): Promise<FileVersion | null> {
    const v = await Binding.SnapshotFile(projectPath, filePath, branch);
    return v ? toVersion(v) : null;
  },

  // 把某文件恢复到指定版本（写回工作树）；后端恢复前自动打 Before-Restore 标签。
  async restoreVersion(projectPath: string, filePath: string, versionId: string, branch: string): Promise<void> {
    await Binding.RestoreVersion(projectPath, filePath, versionId, branch);
  },

  // 某版本与当前工作树文件的 diff（工作树文件不存在视为空 = 删除全部）。
  async versionDiff(projectPath: string, filePath: string, versionId: string): Promise<DiffResult> {
    return toDiff(await Binding.GetVersionDiff(projectPath, filePath, versionId));
  },
} as const;
