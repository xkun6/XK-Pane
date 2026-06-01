import type { Messages } from '@/i18n';

export const webAccessEnUS: Messages = {
  settings: {
    webaccess: {
      title: 'Web access',
      hint: 'Open the address below in a browser on your phone or tablet and scan the QR code to pair, then share sessions with the desktop. The device must be on the same LAN (same Wi-Fi).',
      loading: 'Fetching access address…',
      unavailable: 'Web gateway unavailable: {{message}}',
      retry: 'Retry',
      noLan: 'No LAN address detected. Make sure this machine is connected to Wi-Fi or Ethernet, then hit "Refresh".',
      qrHint: 'Scan with your phone camera or browser to pair and open the UI automatically',
      portLabel: 'Port',
      otherUrls: 'Other interface addresses (pick the one reachable from your phone)',
      copy: 'Copy',
      copied: 'Copied',
      copyAria: 'Copy access address',
      refresh: 'Refresh',
      firewallHint: 'If the first connection from your phone fails, allow this app on "Private networks" in the Windows Firewall prompt.',
      tokenWarning: 'This address embeds a one-time access key (equivalent to a login credential). Do not screenshot, share, or post it publicly.',
    },
  },
};
