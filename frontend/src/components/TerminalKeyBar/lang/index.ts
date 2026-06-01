import { localeRegistry } from '@/i18n';
import { terminalKeyBarZhCN } from './zh-CN';
import { terminalKeyBarEnUS } from './en-US';

let registered = false;

export const registerTerminalKeyBarLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', terminalKeyBarZhCN);
  localeRegistry.extend('en-US', terminalKeyBarEnUS);
  registered = true;
};
