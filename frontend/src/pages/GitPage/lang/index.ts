import { localeRegistry } from '@/i18n';
import { gitPageZhCN } from './zh-CN';
import { gitPageEnUS } from './en-US';

// 把 GitPage 语言包合并到已注册的全局 locale。App 启动时调用一次（幂等）。
let registered = false;

export const registerGitPageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', gitPageZhCN);
  localeRegistry.extend('en-US', gitPageEnUS);
  registered = true;
};
