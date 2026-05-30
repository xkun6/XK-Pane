import { localeRegistry } from '@/i18n';
import { memorysPageZhCN } from './zh-CN';
import { memorysPageEnUS } from './en-US';

// 把 MemorysPage 语言包合并到已注册的全局 locale。App 启动时调用一次（幂等）。
let registered = false;

export const registerMemorysPageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', memorysPageZhCN);
  localeRegistry.extend('en-US', memorysPageEnUS);
  registered = true;
};
