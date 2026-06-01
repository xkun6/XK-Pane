// WebAccessService：Web 网关访问信息的前端入口（仅桌面端用）。
//
// 后端 webapi.Service.GetAccessInfo 返回手机配对所需信息：端口 + 各局域网 IPv4 的配对 URL
// （内嵌一次性 token）。「设置 → Web 访问」页据此渲染二维码。网关未启动时 GetAccessInfo
// 会 reject（"web gateway 未启动"），ViewModel 据此进入 unavailable 态。
//
// 安全注意：urls / primaryUrl 含明文 token，仅在本地可信的桌面 webview 内展示（配对二维码）；
// 绝不把它持久化或回传——它就是给手机扫一次用的配对凭据。
import { Service as Binding } from '@bindings/xkpane/internal/webapi';

export interface AccessInfo {
  port: number;
  urls: string[]; // 每个局域网 IPv4 一条 http://<ip>:<port>/pair?token=...
  primaryUrl: string; // urls 首个；二维码默认用它
}

export const WebAccessService = {
  async getAccessInfo(): Promise<AccessInfo> {
    const a = await Binding.GetAccessInfo();
    return {
      port: Number(a.port ?? 0),
      urls: (a.urls ?? []).filter((u): u is string => typeof u === 'string'),
      primaryUrl: a.primaryUrl ?? '',
    };
  },
} as const;
