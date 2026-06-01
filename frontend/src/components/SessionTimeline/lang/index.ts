import { localeRegistry } from '@/i18n';
import { sessionTimelineZhCN } from './zh-CN';
import { sessionTimelineEnUS } from './en-US';

let registered = false;

export const registerSessionTimelineLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', sessionTimelineZhCN);
  localeRegistry.extend('en-US', sessionTimelineEnUS);
  registered = true;
};
