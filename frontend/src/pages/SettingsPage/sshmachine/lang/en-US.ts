import type { Messages } from '@/i18n';

export const sshMachineSettingsEnUS: Messages = {
  settings: {
    sshmachine: {
      title: 'SSH remote',
      hint: 'Manage reusable SSH remote hosts for launching terminals / CLIs (claude / codex, etc.) on remote machines. Passwords are AES-GCM encrypted in the local database and never echoed back — leave the password field blank when editing to keep the current one. Private keys are referenced by file path (ssh -i), not stored.',
      add: 'Add machine',
      empty: 'No SSH machines yet. Click "Add machine" to configure one.',
      pwStored: 'Password saved',
      auth: {
        key: 'Key',
        agent: 'SSH agent',
        password: 'Password',
      },
      actions: {
        test: 'Test connectivity',
        edit: 'Edit',
        remove: 'Delete',
      },
      form: {
        addTitle: 'Add SSH machine',
        editTitle: 'Edit SSH machine',
        name: 'Name',
        namePlaceholder: 'e.g. Dev server',
        host: 'Host',
        hostPlaceholder: 'Hostname or IP',
        port: 'Port',
        user: 'User',
        userPlaceholder: 'Remote login user (optional)',
        authMethod: 'Auth method',
        identityFile: 'Private key file path',
        identityFilePlaceholder: 'e.g. ~/.ssh/id_ed25519 (blank = default key / agent)',
        password: 'Password',
        passwordPlaceholderNew: 'Login password (stored encrypted, used for auto-answer)',
        passwordPlaceholderEdit: 'Leave blank to keep current password',
        defaultPath: 'Default remote dir',
        defaultPathPlaceholder: 'Auto-cd on connect, e.g. ~/projects (blank = ~)',
        tags: 'Tags',
        tagsPlaceholder: 'Comma-separated, e.g. prod, intranet',
        description: 'Notes',
        save: 'Save',
        cancel: 'Cancel',
      },
      removeConfirm: {
        title: 'Delete SSH machine',
        message: 'Delete machine "{{name}}"? This cannot be undone.',
        ok: 'Delete',
        cancel: 'Cancel',
      },
      error: {
        title: 'SSH machine operation failed',
      },
    },
  },
};
