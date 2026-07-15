import { localeRegistry } from '@/i18n';
import { skillsZhCN } from './zh-CN';
import { skillsEnUS } from './en-US';

let registered = false;

export const registerSkillsLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', skillsZhCN);
  localeRegistry.extend('en-US', skillsEnUS);
  registered = true;
};
