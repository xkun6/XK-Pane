import type { Messages } from '@/i18n';

export const screenshotEnUS: Messages = {
  settings: {
    screenshot: {
      title: 'Screenshot shortcut',
      hint: 'Press the shortcut anywhere to capture a screen region. The saved PNG path is pasted into the active terminal (and copied to clipboard) so Claude Code can reference the image. Windows only.',
      shortcut: {
        label: 'Shortcut',
        recording: 'Press a new combination (Esc to exit)…',
        hint: 'At least one modifier (Ctrl / Alt / Shift / Win) plus one key (A-Z / 0-9 / F1-F24). Registration fails if another app already owns the combination.',
        apply: 'Apply',
        cancel: 'Cancel',
        ariaRecorder: 'Shortcut recorder',
      },
      retention: {
        label: 'Retention days',
        hint: 'Screenshots older than this are cleaned up on each save; set 0 to keep forever.',
      },
      feedback: {
        applying: 'Updating shortcut…',
        applied: 'Shortcut updated',
        failed: 'Failed to update shortcut: {{message}}',
      },
    },
  },
};
