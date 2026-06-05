import { localeRegistry } from '@/i18n';
import { sshMachineSettingsZhCN } from './zh-CN';
import { sshMachineSettingsEnUS } from './en-US';

let registered = false;

export const registerSshMachineSettingsLocales = (): void => {
  if (registered) return;
  localeRegistry.extend('zh-CN', sshMachineSettingsZhCN);
  localeRegistry.extend('en-US', sshMachineSettingsEnUS);
  registered = true;
};
