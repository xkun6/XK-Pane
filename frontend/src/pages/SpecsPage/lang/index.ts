import { localeRegistry } from '@/i18n';
import { specsPageZhCN } from './zh-CN';
import { specsPageEnUS } from './en-US';

// 把 SpecsPage 语言包合并到已注册的全局 locale。App 启动时调用一次（幂等）。
let registered = false;

export const registerSpecsPageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', specsPageZhCN);
  localeRegistry.extend('en-US', specsPageEnUS);
  registered = true;
};
