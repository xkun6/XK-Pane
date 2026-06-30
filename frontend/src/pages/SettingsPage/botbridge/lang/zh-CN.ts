import type { Messages } from '@/i18n';

export const botbridgeZhCN: Messages = {
  settings: {
    botbridge: {
      title: 'Bot 连接器',
      hint: '把当前终端会话镜像到 IM，并支持文本回写与 AskUserQuestion 作答。纯文本平台会降级为「文字编号问答」。会话绑定在 IM 内发送 /sessions 后选择。',
      enabled: '启用',
      platformLabel: '平台',
      platform: {
        feishu: '飞书',
        qqbot: 'QQ 官方机器人',
        dingtalk: '钉钉',
        wecom: '企业微信',
        qq: 'QQ（OneBot）',
        weixin: '微信',
      },
      field: {
        app_id: 'App ID',
        app_secret: 'App Secret',
        client_id: 'Client ID',
        client_secret: 'Client Secret',
        robot_code: '机器人编码',
        card_template_id: '卡片模板 ID',
        bot_id: '机器人 ID',
        bot_secret: '机器人密钥',
        ws_url: 'WebSocket 地址',
        token: '访问令牌',
      },
      secretPlaceholder: '留空则保留已保存密钥',
      connected: '已连接',
      disconnected: '未连接',
      save: '保存',
      saved: '配置已保存',
    },
  },
};
