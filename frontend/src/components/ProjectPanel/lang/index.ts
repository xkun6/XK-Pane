import { localeRegistry } from '@/i18n';
import { projectPanelZhCN } from './zh-CN';
import { projectPanelEnUS } from './en-US';

let registered = false;

export const registerProjectPanelLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', projectPanelZhCN);
  localeRegistry.extend('en-US', projectPanelEnUS);
  registered = true;
};
