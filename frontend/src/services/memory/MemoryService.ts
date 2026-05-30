// MemoryService：记忆（Memory）子系统的前端封装，包 @bindings/xkpane memorysvc。
//
// 后端是 app 级单库（memories 表）。本封装收口 MemorysPage 所需：
// search / get / store / update / remove / stats。tags 跨 IPC 已是 string[]。
import {
  Service as Binding,
  StoreMemoryInput as RawStore,
  UpdateMemoryInput as RawUpdate,
  MemoryQuery as RawQuery,
} from '@bindings/xkpane/internal/services/memorysvc';
import type {
  MemoryDTO as RawMemory,
  MemoryStats as RawStats,
  MemoryQueryResult as RawResult,
} from '@bindings/xkpane/internal/services/memorysvc/models';

// 前端可见的记忆视图（时间字段为 RFC3339 串）。
export interface Memory {
  id: string;
  title: string;
  content: string;
  scope: string; // global | workspace | project | session
  category: string; // decision | lesson | preference | pattern | fact | plan | <custom>
  importance: number; // 1-5
  workspaceName: string | null;
  projectPath: string | null;
  sessionId: string | null;
  tags: string[];
  source: string;
  accessCount: number;
  syncStatus: string;
  isDeleted: boolean;
  createdAt: string;
  updatedAt: string;
  accessedAt: string;
}

export interface MemoryStats {
  total: number;
  byScope: Record<string, number>;
  byCategory: Record<string, number>;
}

export interface MemoryQuery {
  search?: string;
  scope?: string;
  category?: string;
  minImportance?: number;
  workspaceName?: string;
  projectPath?: string;
  sessionId?: string;
  tags?: string[];
  fromDate?: string;
  toDate?: string;
  sortBy?: string; // created_at | updated_at | importance
  limit?: number;
  offset?: number;
}

export interface MemoryQueryResult {
  items: Memory[];
  total: number;
  hasMore: boolean;
}

export interface StoreMemoryInput {
  title: string;
  content: string;
  scope?: string;
  category?: string;
  importance?: number;
  workspaceName?: string;
  projectPath?: string;
  sessionId?: string;
  tags?: string[];
  source?: string;
}

export interface UpdateMemoryInput {
  title?: string;
  content?: string;
  category?: string;
  importance?: number;
  tags?: string[];
}

function toMemory(m: RawMemory): Memory {
  return {
    id: m.id ?? '',
    title: m.title ?? '',
    content: m.content ?? '',
    scope: m.scope ?? 'global',
    category: m.category ?? 'fact',
    importance: m.importance ?? 3,
    workspaceName: m.workspaceName ?? null,
    projectPath: m.projectPath ?? null,
    sessionId: m.sessionId ?? null,
    tags: m.tags ?? [],
    source: m.source ?? 'user',
    accessCount: m.accessCount ?? 0,
    syncStatus: m.syncStatus ?? 'local_only',
    isDeleted: m.isDeleted ?? false,
    createdAt: (m.createdAt as string | null) ?? '',
    updatedAt: (m.updatedAt as string | null) ?? '',
    accessedAt: (m.accessedAt as string | null) ?? '',
  };
}

function toCountMap(map: Record<string, number | undefined> | undefined): Record<string, number> {
  const out: Record<string, number> = {};
  for (const [k, v] of Object.entries(map ?? {})) out[k] = v ?? 0;
  return out;
}

function toResult(r: RawResult): MemoryQueryResult {
  return {
    items: (r.items ?? []).map(toMemory),
    total: r.total ?? 0,
    hasMore: r.hasMore ?? false,
  };
}

export const MemoryService = {
  // 过滤 + 搜索 + 分页查询（软删过滤在后端）。
  async search(q: MemoryQuery): Promise<MemoryQueryResult> {
    return toResult(
      await Binding.Search(
        new RawQuery({
          search: q.search,
          scope: q.scope,
          category: q.category,
          minImportance: q.minImportance,
          workspaceName: q.workspaceName,
          projectPath: q.projectPath,
          sessionId: q.sessionId,
          tags: q.tags,
          fromDate: q.fromDate,
          toDate: q.toDate,
          sortBy: q.sortBy,
          limit: q.limit,
          offset: q.offset,
        }),
      ),
    );
  },

  async get(id: string): Promise<Memory | null> {
    const m = await Binding.Get(id);
    return m ? toMemory(m) : null;
  },

  async store(input: StoreMemoryInput): Promise<Memory> {
    return toMemory(
      await Binding.Store(
        new RawStore({
          title: input.title,
          content: input.content,
          scope: input.scope,
          category: input.category,
          importance: input.importance,
          workspaceName: input.workspaceName,
          projectPath: input.projectPath,
          sessionId: input.sessionId,
          tags: input.tags,
          source: input.source,
        }),
      ),
    );
  },

  async update(id: string, patch: UpdateMemoryInput): Promise<void> {
    await Binding.Update(
      id,
      new RawUpdate({
        title: patch.title,
        content: patch.content,
        category: patch.category,
        importance: patch.importance,
        tags: patch.tags,
      }),
    );
  },

  async remove(id: string): Promise<void> {
    await Binding.Delete(id);
  },

  // 统计（total + byScope + byCategory）。workspace/project 可选限定。
  async stats(workspaceName?: string, projectPath?: string): Promise<MemoryStats> {
    const s: RawStats = await Binding.Stats(workspaceName ?? null, projectPath ?? null);
    return {
      total: s.total ?? 0,
      byScope: toCountMap(s.byScope),
      byCategory: toCountMap(s.byCategory),
    };
  },
} as const;
