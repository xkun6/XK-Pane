import type { Messages } from '@/i18n';

export const mcpEnUS: Messages = {
  settings: {
    mcp: {
      title: 'MCP management',
      hint: 'MCP servers are owned by XK-Pane as the single source of truth (SSOT) and projected per-CLI via the switches below — Claude Code (mcpServers in ~/.claude.json) and Codex ([mcp_servers] in ~/.codex/config.toml). Turning a switch off removes the server from that CLI while XK-Pane keeps the configuration.',
      add: 'Add',
      importCurrent: 'Import from current config',
      empty: 'No MCP servers yet. Click "Import from current config" to get started, or add one manually.',
      noSummary: '—',
      toggles: {
        claude: 'Claude',
        codex: 'Codex',
      },
      actions: {
        edit: 'Edit',
        remove: 'Remove',
      },
      removeConfirm: {
        title: 'Remove MCP server',
        message: 'Remove MCP server "{{name}}"? It will be removed from any CLI where it is enabled. This cannot be undone.',
        ok: 'Remove',
        cancel: 'Cancel',
      },
      importDone: {
        title: 'Import complete',
        message: 'Added {{count}} new MCP server(s).',
      },
      form: {
        addTitle: 'Add MCP server',
        editTitle: 'Edit MCP server',
        name: 'Name',
        namePlaceholder: 'e.g. filesystem',
        nameError: 'Letters, digits, dot, underscore, hyphen only, and non-empty',
        serverJson: 'Server definition JSON',
        serverJsonPlaceholder:
          '{"type": "sse", "url": "http://..."}; pasting {"servers": {...}} / {"mcpServers": {...}} snippets is supported',
        jsonEmptyError: 'Cannot be empty',
        jsonInvalidError: 'Not a valid JSON object',
        jsonMultipleError: 'Multiple server definitions detected; add one at a time',
        enableClaude: 'Enable for Claude Code',
        enableCodex: 'Enable for Codex',
        save: 'Save',
        cancel: 'Cancel',
      },
      error: {
        title: 'MCP server operation failed',
      },
    },
  },
};
