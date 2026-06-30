import type { Messages } from '@/i18n';

export const botbridgeEnUS: Messages = {
  settings: {
    botbridge: {
      title: 'Bot bridge',
      hint: 'Mirror the active terminal session to an IM, with text write-back and AskUserQuestion answers. Text-only platforms degrade to numbered-choice Q&A. Bind sessions in the IM with /sessions.',
      enabled: 'Enabled',
      platformLabel: 'Platform',
      platform: {
        feishu: 'Feishu',
        qqbot: 'QQ Official Bot',
        dingtalk: 'DingTalk',
        wecom: 'WeCom',
        qq: 'QQ (OneBot)',
        weixin: 'WeChat',
      },
      field: {
        app_id: 'App ID',
        app_secret: 'App Secret',
        client_id: 'Client ID',
        client_secret: 'Client Secret',
        robot_code: 'Robot Code',
        card_template_id: 'Card Template ID',
        bot_id: 'Bot ID',
        bot_secret: 'Bot Secret',
        ws_url: 'WebSocket URL',
        token: 'Access Token',
      },
      secretPlaceholder: 'Leave empty to keep the saved secret',
      connected: 'Connected',
      disconnected: 'Disconnected',
      save: 'Save',
      saved: 'Configuration saved',
    },
  },
};
