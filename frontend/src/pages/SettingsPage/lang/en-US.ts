import type { Messages } from '@/i18n';

// SettingsPage top-level English copy: only "frame" layer (eyebrow/title, list, themes).
// Per-subpage copy lives in SettingsPage/<name>/lang/ and is registered by register<Name>Locales.
export const settingsPageEnUS: Messages = {
  settings: {
    eyebrow: 'Settings',
    title: 'Preferences',
    list: {
      personalization: {
        label: 'Personalization',
        description: 'Theme, display preferences, and custom palette',
      },
      language: {
        label: 'Language',
        description: 'Switch the UI language',
      },
      provider: {
        label: 'Providers',
        description: 'Manage API credentials and endpoints for launching CLIs',
      },
      mcp: {
        label: 'MCP management',
        description: 'Manage MCP servers and project them to each CLI',
      },
      skills: {
        label: 'Skills management',
        description: 'Manage Skills and project them to each CLI',
      },
      database: {
        label: 'Data storage',
        description: 'Database location and disk usage',
      },
      knowledge: {
        label: 'Knowledge base',
        description: 'OTA sync & management of the Bedrock ModSDK docs',
      },
      webaccess: {
        label: 'Web access',
        description: 'LAN access from phone / tablet, shared sessions',
      },
      botbridge: {
        label: 'Bot bridge',
        description: 'Mirror terminal sessions to Feishu with write-back',
      },
      screenshot: {
        label: 'Screenshot shortcut',
        description: 'Global capture hotkey and screenshot retention',
      },
    },
    themes: {
      system: { label: 'Follow system', description: 'Tracks the OS light / dark preference' },
      light: { label: 'Light', description: 'Default white theme for daytime use' },
      dark: { label: 'Dark', description: 'Dim grey theme for low-light environments' },
      obsidian: { label: 'Obsidian', description: 'Pure black canvas with violet accent' },
      custom: { label: 'Custom', description: 'Your saved custom palette' },
    },
  },
};
