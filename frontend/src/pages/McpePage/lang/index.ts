import { localeRegistry } from '@/i18n';
import { mcpePageZhCN } from './zh-CN';
import { mcpePageEnUS } from './en-US';

let registered = false;

export const registerMcpePageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', mcpePageZhCN);
  localeRegistry.extend('en-US', mcpePageEnUS);
  registered = true;
};
