import type { Locale } from '../types';

// English (US) — shared / framework-level keys only.
// Page-level copy lives in each page's own lang/ directory.
export const enUS: Locale = {
  code: 'en-US',
  englishName: 'English (US)',
  nativeName: 'English',
  messages: {
    app: {
      title: 'XK-Pane',
    },
    sidebar: {
      brand: 'X',
      navAriaLabel: 'Primary navigation',
    },
    titleBar: {
      controls: {
        minimise: 'Minimise',
        maximise: 'Maximise / Restore',
        close: 'Close',
      },
    },
    route: {
      home: 'Home',
      terminal: 'Terminal',
      files: 'Files',
      git: 'Git',
      history: 'Local History',
      ssh: 'SSH',
      mcpe: 'MC PE',
      todo: 'Todo',
      settings: 'Settings',
    },
    rightPanel: {
      close: 'Close panel',
    },
    terminal: {
      tab: {
        new: 'New terminal',
        close: 'Close tab',
        menu: 'Tab menu',
        rename: 'Rename',
        renameTitle: 'Rename tab',
        renamePlaceholder: 'Tab name',
        pin: 'Pin',
        unpin: 'Unpin',
      },
      view: {
        group: 'Switch view',
        card: 'Cards',
        terminal: 'Terminal',
      },
    },
    common: {
      save: 'Save',
      cancel: 'Cancel',
      remove: 'Remove',
      reset: 'Reset',
      yes: 'Yes',
      no: 'No',
      loading: 'Loading…',
      error: 'Error',
      followSystem: 'Follow system',
      auto: 'Auto',
    },
  },
};
