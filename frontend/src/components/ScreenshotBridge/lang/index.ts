import { localeRegistry } from '@/i18n';
import { screenshotBridgeZhCN } from './zh-CN';
import { screenshotBridgeEnUS } from './en-US';

let registered = false;

export const registerScreenshotBridgeLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', screenshotBridgeZhCN);
  localeRegistry.extend('en-US', screenshotBridgeEnUS);
  registered = true;
};
