import { localeRegistry } from '@/i18n';
import { webWorkspaceTreeZhCN } from './zh-CN';
import { webWorkspaceTreeEnUS } from './en-US';

let registered = false;

export const registerWebWorkspaceTreeLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', webWorkspaceTreeZhCN);
  localeRegistry.extend('en-US', webWorkspaceTreeEnUS);
  registered = true;
};
