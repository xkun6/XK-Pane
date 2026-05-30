// SpecService：需求规格（Spec）的前端封装，包 @bindings/xkpane specsvc。
//
// 后端按 project_path 操作（元数据 app 级单库 + 每项目 .xkpanes/specs/<file>.md 文件）。
// 本封装收口 SpecsPage 所需：list/create/get/getActive/getContent/saveContent/update/remove。
import { Service as Binding } from '@bindings/xkpane/internal/services/specsvc';
import type { Spec as RawSpec } from '@bindings/xkpane/internal/storage/models';

// 前端可见的 Spec 视图（时间字段为 RFC3339 串）。
export interface Spec {
  id: string;
  projectPath: string;
  title: string;
  fileName: string;
  status: string; // draft | active | archived
  todoId: string | null;
  archivedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

function toSpec(s: RawSpec): Spec {
  return {
    id: s.id ?? '',
    projectPath: s.projectPath ?? '',
    title: s.title ?? '',
    fileName: s.fileName ?? '',
    status: s.status ?? 'draft',
    todoId: s.todoId ?? null,
    archivedAt: (s.archivedAt as string | null) ?? null,
    createdAt: (s.createdAt as string | null) ?? '',
    updatedAt: (s.updatedAt as string | null) ?? '',
  };
}

export const SpecService = {
  // 列某项目的 Spec；status 空串 = 全部，按 created_at 倒序。
  async list(projectPath: string, status = ''): Promise<Spec[]> {
    const rows = await Binding.List(projectPath, status);
    return (rows ?? []).map(toSpec);
  },

  // 新建 Spec（后端插元数据 + 从模板生成 .md 文件）。
  async create(projectPath: string, title: string): Promise<Spec> {
    return toSpec(await Binding.Create(projectPath, title));
  },

  async get(id: string): Promise<Spec | null> {
    const s = await Binding.Get(id);
    return s ? toSpec(s) : null;
  },

  async getActive(projectPath: string): Promise<Spec | null> {
    const s = await Binding.GetActive(projectPath);
    return s ? toSpec(s) : null;
  },

  // 读 / 写 .md 文件内容。
  async getContent(projectPath: string, id: string): Promise<string> {
    return Binding.GetContent(projectPath, id);
  },
  async saveContent(projectPath: string, id: string, content: string): Promise<void> {
    await Binding.SaveContent(projectPath, id, content);
  },

  // 改元数据（title / status，省略传 null 不改）。返回更新后的 Spec（不存在则 null）。
  async update(id: string, patch: { title?: string; status?: string }): Promise<Spec | null> {
    const s = await Binding.Update(id, patch.title ?? null, patch.status ?? null);
    return s ? toSpec(s) : null;
  },

  async remove(projectPath: string, id: string): Promise<void> {
    await Binding.Delete(projectPath, id);
  },
} as const;
