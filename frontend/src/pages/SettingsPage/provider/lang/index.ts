import { localeRegistry } from '@/i18n';
import { providerSettingsZhCN } from './zh-CN';
import { providerSettingsEnUS } from './en-US';

let registered = false;

export const registerProviderSettingsLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', providerSettingsZhCN);
  localeRegistry.extend('en-US', providerSettingsEnUS);
  registered = true;
};
