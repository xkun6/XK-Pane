import type { Messages } from '@/i18n';

export const providerSettingsEnUS: Messages = {
  settings: {
    provider: {
      tabs: {
        claude: 'Claude Code',
        codex: 'Codex',
        other: 'Other CLIs',
      },
      // cc-switch style global config Provider (shared by the Claude Code / Codex tabs).
      global: {
        add: 'Add',
        importCurrent: 'Import from current config',
        hint: 'Switching writes directly to this CLI\'s global config file, affecting every session of this CLI launched afterwards (including outside XK-Pane).',
        empty: 'No providers yet. Click "Import from current config" to get started quickly.',
        currentBadge: 'Current',
        category: {
          official: 'Official',
          custom: 'Custom',
          imported: 'Imported',
        },
        actions: {
          switch: 'Switch',
          edit: 'Edit',
          remove: 'Delete',
        },
        removeConfirm: {
          title: 'Delete provider',
          message: 'Delete provider "{{name}}"? This cannot be undone.',
          ok: 'Delete',
          cancel: 'Cancel',
        },
        form: {
          addTitle: 'Add {{app}} provider',
          editTitle: 'Edit {{app}} provider',
          name: 'Name',
          namePlaceholder: 'e.g. Official direct',
          notes: 'Notes (optional)',
          notesPlaceholder: 'A note for yourself',
          currentEditWarning: 'This is the {{app}} provider currently in use — saving will immediately sync to the global config file.',
          template: 'Template',
          templates: {
            blank: 'Blank',
            apiKey: 'API key (direct)',
            thirdParty: 'Third-party relay',
          },
          templateOverwriteConfirm: {
            title: 'Overwrite current content',
            message: 'Applying a template will overwrite what you\'ve already typed in the editor below. Continue?',
            ok: 'Overwrite',
            cancel: 'Cancel',
          },
          settingsJson: 'settings.json config',
          jsonEmptyError: 'Required — at least {}',
          jsonInvalidError: 'Not a valid JSON object',
          authJson: 'auth.json (login state, optional)',
          authJsonHelper: 'Leave blank = do not manage login state (switching to this provider will not touch auth.json)',
          configToml: 'config.toml',
          save: 'Save',
          cancel: 'Cancel',
        },
      },
      // "Other CLIs" tab (session-injection types: gemini / kimi / glm / cursor / opencode).
      hint: 'Manage the access credentials used to launch CLIs (API key / endpoint). Keys are AES-GCM encrypted in the local database and never echoed back in plaintext — leave the key field blank when editing to keep the existing one.',
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
        namePlaceholder: 'e.g. My Gemini',
        type: 'Type',
        apiKey: 'API key',
        apiKeyPlaceholderNew: 'Paste your API key',
        apiKeyPlaceholderEdit: 'Leave blank to keep current key',
        baseUrl: 'Base URL (optional)',
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
