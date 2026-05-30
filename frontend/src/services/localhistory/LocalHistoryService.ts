// LocalHistoryService：本地历史（文件版本快照）的前端封装，包 @bindings/xkpane localhistorysvc。
//
// 后端是每项目独立 db（D10），service 按 projectPath 操作。本封装当前收口历史面板第一阶段
// 所需子集：recentChanges（跨文件最近变更）/ snapshotFile（手动快照）/ restoreVersion（回滚）。
// diff 查看（getVersionDiff / getVersionsDiff）+ 版本列表 + 标签管理留后续单元（diff 渲染较重）。
import { Service as Binding } from '@bindings/xkpane/internal/services/localhistorysvc';
import type { RecentChange as RawRecent, FileVersion as RawVersion } from '@bindings/xkpane/internal/localhistory/models';

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
} as const;
