import { localeRegistry } from '@/i18n';
import { botbridgeZhCN } from './zh-CN';
import { botbridgeEnUS } from './en-US';

let registered = false;

export const registerBotbridgeLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', botbridgeZhCN);
  localeRegistry.extend('en-US', botbridgeEnUS);
  registered = true;
};

