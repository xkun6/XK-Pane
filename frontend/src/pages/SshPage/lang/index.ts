import { localeRegistry } from '@/i18n';
import { sshPageZhCN } from './zh-CN';
import { sshPageEnUS } from './en-US';

let registered = false;

// 注册 SSH 页文案（ssh.* 命名空间）。在 App.tsx 启动时调用一次，幂等。
export const registerSshPageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', sshPageZhCN);
  localeRegistry.extend('en-US', sshPageEnUS);
  registered = true;
};
