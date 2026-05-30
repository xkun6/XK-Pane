import { localeRegistry } from '@/i18n';
import { paneTreeZhCN } from './zh-CN';
import { paneTreeEnUS } from './en-US';

let registered = false;

export const registerPaneTreeLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', paneTreeZhCN);
  localeRegistry.extend('en-US', paneTreeEnUS);
  registered = true;
};
