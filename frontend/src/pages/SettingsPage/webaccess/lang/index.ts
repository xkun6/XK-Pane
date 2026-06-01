import { localeRegistry } from '@/i18n';
import { webAccessZhCN } from './zh-CN';
import { webAccessEnUS } from './en-US';

let registered = false;

export const registerWebAccessLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', webAccessZhCN);
  localeRegistry.extend('en-US', webAccessEnUS);
  registered = true;
};
