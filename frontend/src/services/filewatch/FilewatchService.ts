// FilewatchService：文件浏览器磁盘变更监听的前端门面，包 @bindings/xkpane filewatch service。
//
// 桌面专属（监听本机文件系统）。Web 端无文件系统，静默 no-op（自动刷新只是体验增强，缺失不致命）。
// 后端单活动根：Watch(root) 替换上一个监听；前端在 rootDir 变化时调 watch、卸载时调 unwatch。
import { Service as Binding } from '@bindings/xkpane/internal/services/filewatch';
import { isDesktop } from '@/services/transport';

export const FilewatchService = {
  // 开始监听 root（替换上一个）；失败仅失去自动刷新，不抛给上层。
  async watch(root: string): Promise<void> {
    if (!isDesktop || !root) return;
    try {
      await Binding.Watch(root);
    } catch {
      /* 监听失败：仅失去自动刷新，非关键 */
    }
  },

  // 停止当前监听。
  async unwatch(): Promise<void> {
    if (!isDesktop) return;
    try {
      await Binding.Unwatch();
    } catch {
      /* ignore */
    }
  },
} as const;
