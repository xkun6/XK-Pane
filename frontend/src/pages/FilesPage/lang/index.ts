import { localeRegistry } from '@/i18n';
import { filesPageZhCN } from './zh-CN';
import { filesPageEnUS } from './en-US';

let registered = false;

export const registerFilesPageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', filesPageZhCN);
  localeRegistry.extend('en-US', filesPageEnUS);
  registered = true;
};
