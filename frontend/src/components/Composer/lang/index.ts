import { localeRegistry } from '@/i18n';
import { composerZhCN } from './zh-CN';
import { composerEnUS } from './en-US';

let registered = false;

export const registerComposerLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', composerZhCN);
  localeRegistry.extend('en-US', composerEnUS);
  registered = true;
};
