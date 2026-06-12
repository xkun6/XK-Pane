import { localeRegistry } from '@/i18n';
import { screenshotZhCN } from './zh-CN';
import { screenshotEnUS } from './en-US';

let registered = false;

export const registerScreenshotLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', screenshotZhCN);
  localeRegistry.extend('en-US', screenshotEnUS);
  registered = true;
};
