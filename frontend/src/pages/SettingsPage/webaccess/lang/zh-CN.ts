import type { Messages } from '@/i18n';

export const webAccessZhCN: Messages = {
  settings: {
    webaccess: {
      title: 'Web 访问',
      hint: '在手机或平板的浏览器里打开下面的地址、扫码配对后，即可与桌面端共享会话。手机需与本机处于同一局域网（同一 Wi-Fi）。',
      loading: '正在获取访问地址…',
      unavailable: 'Web 网关不可用：{{message}}',
      retry: '重试',
      noLan: '未检测到局域网地址。请确认本机已连接 Wi-Fi 或有线网络后点「刷新地址」重试。',
      qrHint: '用手机相机或浏览器扫码，即可自动配对并打开界面',
      portLabel: '端口',
      otherUrls: '其它网卡地址（有多个网段时，挑手机能访问的那条）',
      copy: '复制',
      copied: '已复制',
      copyAria: '复制访问地址',
      refresh: '刷新地址',
      firewallHint: '首次从手机访问若连不上，请在 Windows 防火墙弹窗里允许本应用访问「专用网络」。',
      tokenWarning: '该地址内含一次性访问密钥（等同登录凭据），请勿截图外传或贴到公开场合。',
    },
  },
};
