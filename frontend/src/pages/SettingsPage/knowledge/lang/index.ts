import { localeRegistry } from '@/i18n';
import { knowledgeZhCN } from './zh-CN';
import { knowledgeEnUS } from './en-US';

let registered = false;

export const registerKnowledgeLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', knowledgeZhCN);
  localeRegistry.extend('en-US', knowledgeEnUS);
  registered = true;
};
