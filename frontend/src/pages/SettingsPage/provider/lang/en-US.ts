import type { Messages } from '@/i18n';

export const providerSettingsEnUS: Messages = {
  settings: {
    provider: {
      title: 'Providers',
      hint: 'Manage the access credentials used to launch CLIs (API key / endpoint / cloud config). Keys are AES-GCM encrypted in the local database and never echoed back in plaintext — leave the key field blank when editing to keep the existing one.',
      add: 'Add provider',
      empty: 'No providers yet. Click "Add provider" to configure one.',
      defaultBadge: 'Default',
      keySet: 'Key set',
      actions: {
        setDefault: 'Set as default',
        edit: 'Edit',
        remove: 'Delete',
      },
      types: {
        anthropic: 'Anthropic',
        bedrock: 'AWS Bedrock',
        vertex: 'Google Vertex',
        proxy: 'Proxy (Anthropic-compatible)',
        config_profile: 'Config dir (ccswitch)',
        open_ai: 'OpenAI / Codex',
        gemini: 'Gemini',
        kimi: 'Kimi',
        glm: 'GLM (Zhipu)',
        cursor: 'Cursor',
        opencode: 'OpenCode',
      },
      form: {
        addTitle: 'Add provider',
        editTitle: 'Edit provider',
        name: 'Name',
        namePlaceholder: 'e.g. My Anthropic',
        type: 'Type',
        apiKey: 'API key',
        apiKeyPlaceholderNew: 'Paste your API key',
        apiKeyPlaceholderEdit: 'Leave blank to keep current key',
        baseUrl: 'Base URL (optional)',
        region: 'Region',
        awsProfile: 'AWS profile',
        projectId: 'GCP project ID',
        configDir: 'Config directory path',
        isDefault: 'Set as default provider',
        save: 'Save',
        cancel: 'Cancel',
      },
      removeConfirm: {
        title: 'Delete provider',
        message: 'Delete provider "{{name}}"? This cannot be undone.',
        ok: 'Delete',
        cancel: 'Cancel',
      },
      error: {
        title: 'Provider operation failed',
      },
    },
  },
};
