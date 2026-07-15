import type { Messages } from '@/i18n';

export const databaseEnUS: Messages = {
  settings: {
    database: {
      title: 'Data storage',
      hint: 'All preferences and settings live in a SQLite database. You can move the database file to any location you trust; switching migrates the data automatically.',
      currentPathLabel: 'Current location',
      defaultPathLabel: 'Default location',
      sizeLabel: 'Disk usage',
      customBadge: 'Custom',
      defaultBadge: 'Default',
      actions: {
        change: 'Change location…',
        reset: 'Reset to default',
      },
      dialog: {
        title: 'Choose a new database location',
        message: 'Existing data will be copied to the chosen location.',
        confirm: 'Save here',
        filterDb: 'SQLite databases (*.db, *.sqlite)',
      },
      confirmReset: {
        title: 'Restore default location',
        message: 'The database file will be moved back to the platform default directory. Continue?',
        ok: 'Restore',
        cancel: 'Cancel',
      },
      confirmClear: {
        title: 'Clear {{label}}',
        message: 'This will delete all {{rows}} rows in "{{label}}". This action cannot be undone. Continue?',
        ok: 'Clear',
        cancel: 'Cancel',
      },
      feedback: {
        changing: 'Migrating database…',
        changed: 'Switched to the new location',
        resetting: 'Restoring default location…',
        reset: 'Restored to default location',
        clearing: 'Clearing "{{label}}"…',
        cleared: 'Cleared "{{label}}"',
        failed: 'Operation failed: {{message}}',
      },
      charts: {
        share: {
          title: 'Space share',
          hint: 'Relative footprint of each table inside the database.',
        },
        bytes: {
          title: 'Byte usage',
          hint: 'Absolute bytes per table (including index pages).',
        },
        empty: 'No data yet',
        estimatedNote: 'Some entries are estimated (extrapolated from row counts).',
      },
      tables: {
        title: 'Tables',
        empty: 'No tables to display.',
        meta: '{{rows}} rows · {{size}}',
        clear: 'Clear',
        clearAria: 'Clear {{label}}',
        protected: 'Protected',
        protectedAria: '{{label}} is protected from one-click clearing',
        estimatedTag: 'estimated',
        preferences: {
          label: 'Behavior preferences',
          description: 'UI toggles and visibility settings — safe to rebuild.',
        },
        appSettings: {
          label: 'App settings',
          description: 'Theme choice / custom palette / locale. Affects visual identity, not eligible for bulk clear.',
        },
        workspaces: {
          label: 'Workspaces',
          description: 'Organizational containers for projects (tier 1 of the two-level tree). Holds membership / order, not eligible for bulk clear.',
        },
        workspaceProjects: {
          label: 'Workspace projects',
          description: 'Project nodes under a workspace (tier 2 of the two-level tree).',
        },
        mcPackages: {
          label: 'MC PE packages',
          description: 'MC PE dev package configs (project / launcher / world / debug hotkeys). Curated, not eligible for bulk clear.',
        },
        recentDirectories: {
          label: 'Recent directories',
          description: 'Directories recently opened in the file browser — safe to rebuild.',
        },
        agentProviders: {
          label: 'CLI providers',
          description: 'Full config snapshots for Claude Code / Codex etc. (including encrypted credentials). Not eligible for bulk clear.',
        },
        agentMcpServers: {
          label: 'MCP servers',
          description: 'MCP server definitions (SSOT), projected to each CLI via switches. Not eligible for bulk clear.',
        },
        agentSkills: {
          label: 'Skills',
          description: 'Skill metadata (SSOT), projected to each CLI via switches. Not eligible for bulk clear.',
        },
      },
    },
  },
};
