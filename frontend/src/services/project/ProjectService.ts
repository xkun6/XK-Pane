// ProjectService：项目登记表（projects 表）的前端封装。
//
// 包 @bindings/xkpane project 服务的 6 个方法。后端是 cc-pane 扁平 Project 实体的
// 忠实移植——path 即 PTY cwd / 唯一操作目标（铁律 D7），原样存储不规范化；
// List 按 created_at 倒序（最新在前）；alias 设/清由后端 trim→NULL 归一。
//
// 注意：cc-pane 前端 add 后还会 best-effort 触发 init_project_history（Local
// History 文件监视）——那是独立子系统，本 MVP 不接，留 Phase 4/5。
import { Service as Binding } from '@bindings/xkpane/internal/services/project';
import type { Project as RawProject } from '@bindings/xkpane/internal/storage/models';
import { httpClient, isDesktop } from '@/services/transport';

// Project 对外的纯接口（与生成类解耦，供 ViewModel 消费）。对齐后端
// storage.Project 的 json camelCase。createdAt/updatedAt 是后端 time.Time 序列化的
// RFC3339 字符串（生成类型为 any，运行时为字符串）。
export interface Project {
  id: string;
  name: string;
  path: string; // = PTY cwd / 唯一操作目标（铁律 D7）
  alias: string | null; // null = 未设别名；显示名取 alias || basename(path)
  createdAt: string;
  updatedAt: string;
}

// toProject 把 binding 返回的模型实例规整成纯接口（容错缺失字段）。
function toProject(p: RawProject): Project {
  return {
    id: p.id ?? '',
    name: p.name ?? '',
    path: p.path ?? '',
    alias: p.alias ?? null,
    createdAt: typeof p.createdAt === 'string' ? p.createdAt : '',
    updatedAt: typeof p.updatedAt === 'string' ? p.updatedAt : '',
  };
}

export const ProjectService = {
  // 全部项目，最新在前（后端已 created_at DESC 排序）。
  // 浏览器（viewer）走 GET /api/projects（只读白名单，供 Web 启动器选合法 projectPath）；
  // add/remove/updateName/updateAlias 等写方法不在 Web 暴露（fail-closed §9，桌面专属）。
  async list(): Promise<Project[]> {
    if (!isDesktop) {
      const res = await httpClient.get<{ projects?: RawProject[] }>('/api/projects');
      return (res.projects ?? []).map(toProject);
    }
    const rows = await Binding.List();
    return (rows ?? []).map(toProject);
  },

  // 登记一个项目目录（path 须绝对、存在且为目录；重复 path 报 "Project already exists"）。
  async add(path: string): Promise<Project> {
    return toProject(await Binding.Add(path));
  },

  // 按 id 查；不存在返回 null（非错误）。
  async get(id: string): Promise<Project | null> {
    const p = await Binding.Get(id);
    return p ? toProject(p) : null;
  },

  // 按 id 删除（无匹配抛 "Project does not exist"）。
  async remove(id: string): Promise<void> {
    await Binding.Remove(id);
  },

  // 改名（空名/纯空白会被后端拒绝；存储未 trim 原值）。
  async updateName(id: string, name: string): Promise<void> {
    await Binding.UpdateName(id, name);
  },

  // 设/清别名：传 null 或空白清除，否则存 trim 后的值（后端归一）。
  async updateAlias(id: string, alias: string | null): Promise<void> {
    await Binding.UpdateAlias(id, alias);
  },
} as const;
