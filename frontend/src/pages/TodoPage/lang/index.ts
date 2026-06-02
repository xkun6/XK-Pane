import { localeRegistry } from '@/i18n';
import { todoPageZhCN } from './zh-CN';
import { todoPageEnUS } from './en-US';

// 把 TodoPage 语言包合并到已注册的全局 locale。App 启动时调用一次（幂等）。
let registered = false;

export const registerTodoPageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', todoPageZhCN);
  localeRegistry.extend('en-US', todoPageEnUS);
  registered = true;
};
