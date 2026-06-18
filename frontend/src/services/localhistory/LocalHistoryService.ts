// LocalHistoryService：本地历史（文件版本快照）的前端封装，包 @bindings/xkpane localhistorysvc。
//
// 后端是每项目独立 db（D10），service 按 projectPath 操作。本封装当前收口历史面板第一阶段
// 所需子集：recentChanges（跨文件最近变更）/ snapshotFile（手动快照）/ restoreVersion（回滚）。
// diff 查看（getVersionDiff / getVersionsDiff）+ 版本列表 + 标签管理留后续单元（diff 渲染较重）。
import { Service as Binding } from '@bindings/xkpane/internal/services/localhistorysvc';
// 值导入 HistoryLabel 模型类：putLabel 用 new LabelModel(...) 构造，确保形状与后端绑定一致。
import { HistoryLabel as LabelModel } from '@bindings/xkpane/internal/localhistory/models';
import type {
  RecentChange as RawRecent,
  FileVersion as RawVersion,
  DiffResult as RawDiff,
  DiffHunk as RawHunk,
  DiffLine as RawLine,
  InlineChange as RawInline,
  HistoryConfig as RawConfig,
  HistoryStats as RawStats,
  HistoryLabel as RawLabel,
  LabelFileSnapshot as RawLabelSnap,
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

// 每项目历史配置（保留期 / 数量 / 大小上限 / 忽略规则）。
export interface HistoryConfig {
  enabled: boolean;
  ignorePatterns: string[];
  maxVersionsPerFile: number;
  maxAgeDays: number;
  maxFileSize: number; // 单文件大小上限（字节）
  maxTotalSize: number; // 项目级总存储上限（字节）
  minSaveIntervalSecs: number; // 同文件同分支最小保存间隔（秒）
}

// 存储概要统计（清理前后对比）。
export interface HistoryStats {
  versionCount: number;
  fileCount: number;
  totalBlobSize: number; // blobs 目录总字节
}

// 标签来源（彩色 badge + 文案）。对齐 cc-pane Label.source 取值。
export type LabelSource =
  | 'git_commit'
  | 'claude_session'
  | 'user'
  | 'build'
  | 'restore'
  | 'branch_switch';

// 标签关联的「文件 → 版本」引用。
export interface LabelFileSnapshot {
  filePath: string;
  versionId: string;
}

// 一个标签（组合视图：元数据 + 文件快照列表）。对齐后端 localhistory.HistoryLabel。
export interface HistoryLabel {
  id: string;
  name: string;
  labelType: string; // "auto" | "manual"
  source: string; // 见 LabelSource
  timestamp: string; // RFC3339
  branch: string;
  fileSnapshots: LabelFileSnapshot[];
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

function toConfig(c: RawConfig): HistoryConfig {
  return {
    enabled: c.enabled ?? true,
    ignorePatterns: c.ignorePatterns ?? [],
    maxVersionsPerFile: c.maxVersionsPerFile ?? 0,
    maxAgeDays: c.maxAgeDays ?? 0,
    maxFileSize: c.maxFileSize ?? 0,
    maxTotalSize: c.maxTotalSize ?? 0,
    minSaveIntervalSecs: c.minSaveIntervalSecs ?? 0,
  };
}

function toStats(s: RawStats): HistoryStats {
  return {
    versionCount: s.versionCount ?? 0,
    fileCount: s.fileCount ?? 0,
    totalBlobSize: s.totalBlobSize ?? 0,
  };
}

function toLabelSnap(s: RawLabelSnap): LabelFileSnapshot {
  return { filePath: s.filePath ?? '', versionId: s.versionId ?? '' };
}

function toLabel(l: RawLabel): HistoryLabel {
  return {
    id: l.id ?? '',
    name: l.name ?? '',
    labelType: l.labelType ?? '',
    source: l.source ?? '',
    timestamp: l.timestamp ?? '',
    branch: l.branch ?? '',
    fileSnapshots: (l.fileSnapshots ?? []).map(toLabelSnap),
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

  // 列出某文件的所有版本（后端按 id 升序 = 时序；UI 通常 reverse 成最新在上）。
  async listVersions(projectPath: string, filePath: string): Promise<FileVersion[]> {
    const rows = await Binding.ListVersions(projectPath, filePath);
    return (rows ?? []).map(toVersion);
  },

  // 列出「最新版本为删除标记」的文件（已删但有历史可恢复）。
  async listDeletedFiles(projectPath: string): Promise<FileVersion[]> {
    const rows = await Binding.ListDeletedFiles(projectPath);
    return (rows ?? []).map(toVersion);
  },

  // 取某版本的文本内容（gzip 兼容读）。
  async versionContent(projectPath: string, filePath: string, versionId: string): Promise<string> {
    return Binding.GetVersionContent(projectPath, filePath, versionId);
  },

  // 某版本与当前工作树文件的 diff（工作树文件不存在视为空 = 删除全部）。
  async versionDiff(projectPath: string, filePath: string, versionId: string): Promise<DiffResult> {
    return toDiff(await Binding.GetVersionDiff(projectPath, filePath, versionId));
  },

  // 两个版本之间的 diff（相邻版本比较用）。
  async versionsDiff(
    projectPath: string,
    filePath: string,
    oldVersionId: string,
    newVersionId: string,
  ): Promise<DiffResult> {
    return toDiff(await Binding.GetVersionsDiff(projectPath, filePath, oldVersionId, newVersionId));
  },

  // 任意两段文本的 diff（纯函数，复用 ComputeDiff）；供「与剪贴板比较」等就地比对。
  async diffText(oldText: string, newText: string): Promise<DiffResult> {
    return toDiff(await Binding.DiffText(oldText, newText));
  },

  // 某文件有版本记录的所有分支（分支筛选下拉数据源）；无历史 → 空数组。
  async getFileBranches(projectPath: string, filePath: string): Promise<string[]> {
    const rows = await Binding.GetFileBranches(projectPath, filePath);
    return (rows ?? []).filter((b): b is string => typeof b === 'string');
  },

  // 列出所有标签（timestamp 倒序），含各自文件快照。
  async listLabels(projectPath: string): Promise<HistoryLabel[]> {
    const rows = await Binding.ListLabels(projectPath);
    return (rows ?? []).map(toLabel);
  },

  // upsert 一个标签（手动打标签）。前端构造（source='user' / labelType='manual'）。
  async putLabel(projectPath: string, label: HistoryLabel): Promise<void> {
    await Binding.PutLabel(projectPath, new LabelModel(label));
  },

  // 把某标签捕获的所有文件快照恢复到工作树，返回成功恢复的相对路径列表（恢复前自动打 Before-Rollback 标签）。
  async restoreToLabel(projectPath: string, labelId: string, branch: string): Promise<string[]> {
    const rows = await Binding.RestoreToLabel(projectPath, labelId, branch);
    return (rows ?? []).filter((p): p is string => typeof p === 'string');
  },

  // 删除标签及其快照。
  async deleteLabel(projectPath: string, labelId: string): Promise<void> {
    await Binding.DeleteLabel(projectPath, labelId);
  },

  // 读项目历史配置；无历史项目返回默认配置（后端不创建 .xkpanes/）。
  async getConfig(projectPath: string): Promise<HistoryConfig> {
    return toConfig(await Binding.GetConfig(projectPath));
  },

  // 写项目历史配置（显式动作，后端按需创建历史存储）。
  async saveConfig(projectPath: string, cfg: HistoryConfig): Promise<void> {
    await Binding.SaveConfig(projectPath, cfg);
  },

  // 按配置清理（过期 + 超量 + 超总大小 + 孤儿 blob 回收）；无历史项目为 no-op。
  async cleanup(projectPath: string): Promise<void> {
    await Binding.Cleanup(projectPath);
  },

  // 存储概要（版本数 / 文件数 / blob 总字节）；无历史 → 全 0。
  async getStats(projectPath: string): Promise<HistoryStats> {
    return toStats(await Binding.GetStats(projectPath));
  },
} as const;
