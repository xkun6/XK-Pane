// WorkspaceTree 项目节点的显示派生（本地 vs SSH 远程统一入口）。
// 抽出来避免 WorkspaceTree.tsx / WorkspaceProjectItem.tsx 各写一份 basename/displayName。
import type { WorkspaceProject } from '@/services';

// 路径末段（兼容 / 与 \，去尾分隔符）—— 本地项目无别名时的显示名。
function basename(path: string): string {
  const norm = path.replace(/[\\/]+$/, '');
  const idx = Math.max(norm.lastIndexOf('/'), norm.lastIndexOf('\\'));
  return idx >= 0 ? norm.slice(idx + 1) : norm;
}

// SSH 远程项目的友好地址：user@host[:remotePath]（user 空则省略 @，remotePath 空则省略 :path）。
export function sshAddress(p: WorkspaceProject): string {
  if (!p.ssh) return '';
  const base = p.ssh.user ? `${p.ssh.user}@${p.ssh.host}` : p.ssh.host;
  return p.ssh.remotePath ? `${base}:${p.ssh.remotePath}` : base;
}

// 项目显示名：别名优先；SSH 用 user@host；本地用路径末段。
export function projectDisplayName(p: WorkspaceProject): string {
  if (p.alias) return p.alias;
  if (p.ssh) return p.ssh.user ? `${p.ssh.user}@${p.ssh.host}` : p.ssh.host;
  return basename(p.path);
}

// 项目副标题（行内第二行）：SSH 显示地址；本地显示原始路径。
export function projectSubtitle(p: WorkspaceProject): string {
  return p.ssh ? sshAddress(p) : p.path;
}
