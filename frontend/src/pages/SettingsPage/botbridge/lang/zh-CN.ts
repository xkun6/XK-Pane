import type { Messages } from '@/i18n';

export const botbridgeZhCN: Messages = {
  settings: {
    botbridge: {
      title: 'Bot 连接器',
      hint: '通过飞书把当前终端会话镜像到 IM，并支持文本回写与 AskUserQuestion 按钮作答。会话绑定在飞书内发送 /sessions 后选择。',
      enabled: '启用',
      platform: '平台',
      platformFeishu: '飞书',
      appId: 'App ID',
      appSecret: 'App Secret',
      secretPlaceholder: '留空则保留已保存密钥',
      connected: '已连接',
      disconnected: '未连接',
      save: '保存',
      saved: '配置已保存',
    },
  },
};

