import { localeRegistry } from '@/i18n';
import { instructionsZhCN } from './zh-CN';
import { instructionsEnUS } from './en-US';

let registered = false;

export const registerInstructionsLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', instructionsZhCN);
  localeRegistry.extend('en-US', instructionsEnUS);
  registered = true;
};
