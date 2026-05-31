import { localeRegistry } from '@/i18n';
import { historyPageZhCN } from './zh-CN';
import { historyPageEnUS } from './en-US';

// 把 HistoryPage 语言包合并到已注册的全局 locale。App 启动时调用一次（幂等）。
let registered = false;

export const registerHistoryPageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', historyPageZhCN);
  localeRegistry.extend('en-US', historyPageEnUS);
  registered = true;
};
