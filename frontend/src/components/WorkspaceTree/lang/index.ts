import { localeRegistry } from '@/i18n';
import { workspaceTreeZhCN } from './zh-CN';
import { workspaceTreeEnUS } from './en-US';

let registered = false;

export const registerWorkspaceTreeLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', workspaceTreeZhCN);
  localeRegistry.extend('en-US', workspaceTreeEnUS);
  registered = true;
};
