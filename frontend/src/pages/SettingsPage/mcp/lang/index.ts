import { localeRegistry } from '@/i18n';
import { mcpZhCN } from './zh-CN';
import { mcpEnUS } from './en-US';

let registered = false;

export const registerMcpLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', mcpZhCN);
  localeRegistry.extend('en-US', mcpEnUS);
  registered = true;
};
