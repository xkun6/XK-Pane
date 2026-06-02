import { useMemo } from 'react';
import type { Project } from '@/services';
import { isWindows } from '@/shared/platform';
import { useWorkspaces } from '@/workspace';
import { useProjects } from './ProjectProvider';

// useRegisteredProjects：「已登记项目」并集 —— 全局扁平 projects 表 ∪ 各 workspace 下的
// workspace_projects，按 path 去重。镜像后端 app.go registeredProjectPaths 的并集语义。
//
// 为何并集：Git / 本地历史页里的"项目"语义是"任何已登记的目录"，不应限于扁平登记表 ——
// 否则用户从侧边栏「导入项目」进 workspace 的项目永远不会出现在这两页（两表脱节）。
//
// 顺序：扁平项目在前（后端 created_at DESC），其后工作空间项目（树序）；同 path 首次出现者胜。
// workspace_projects 无 name 字段，合成 Project 时 name 取 basename(path)（与 WorkspaceTree
// 的 `alias ?? basename(path)` 显示一致）；createdAt/updatedAt 置空串（消费方 Git/历史页不读）。

// basename：取路径最后一段（兼容 / 与 \），空则回退原串。
function basename(path: string): string {
  const segs = path.split(/[\\/]/).filter(Boolean);
  return segs.at(-1) ?? path;
}

// normalizeProjectPath：仅用于去重比较（不改存储/显示值）。统一斜杠 + 去尾斜杠；
// Windows 大小写不敏感 → 转小写。对齐后端 workspace.normalizeProjectPath。
function normalizeProjectPath(path: string): string {
  let s = path.replace(/\\/g, '/').replace(/\/+$/, '');
  if (isWindows) s = s.toLowerCase();
  return s;
}

export function useRegisteredProjects(): Project[] {
  const { projects } = useProjects();
  const { workspaces } = useWorkspaces();
  return useMemo(() => {
    const seen = new Set<string>();
    const out: Project[] = [];
    const push = (p: Project): void => {
      if (!p.path) return;
      const key = normalizeProjectPath(p.path);
      if (seen.has(key)) return;
      seen.add(key);
      out.push(p);
    };
    for (const p of projects) push(p);
    for (const ws of workspaces) {
      for (const wp of ws.projects) {
        push({
          id: wp.id,
          name: basename(wp.path),
          path: wp.path,
          alias: wp.alias,
          createdAt: '',
          updatedAt: '',
        });
      }
    }
    return out;
  }, [projects, workspaces]);
}
