import { localeRegistry } from '@/i18n';
import { terminalPageZhCN } from './zh-CN';
import { terminalPageEnUS } from './en-US';

let registered = false;

export const registerTerminalPageLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', terminalPageZhCN);
  localeRegistry.extend('en-US', terminalPageEnUS);
  registered = true;
};
