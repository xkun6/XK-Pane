import { localeRegistry } from '@/i18n';
import { sessionsPageZhCN } from './zh-CN';
import { sessionsPageEnUS } from './en-US';

// 把 SessionsPage 语言包合并到已注册的全局 locale。App 启动时调用一次（幂等）。
let registered = false;

export const registerSessionsPageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', sessionsPageZhCN);
  localeRegistry.extend('en-US', sessionsPageEnUS);
  registered = true;
};
