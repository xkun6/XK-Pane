// transport 出口 + 运行环境探针（P7 §2 / M2）。
//   isDesktop：探针用 **window.__XKPANE_WEB__** —— 由 webapi 网关在所托管 HTML 注入（见
//     internal/webapi/http.go webMarker）。**不能**用 `'_wails' in window`：@wailsio/runtime 在
//     任意平台（含浏览器）的模块顶层都会 `window._wails = window._wails || {}`（本文件经 wails.ts
//     静态 import 了它），故 _wails 在浏览器恒存在 → 误判 isDesktop=true → 整条 Web 路径失效、
//     viewer 守护失灵。改用网关注入的标记：经网关访问的浏览器一定带标记 → viewer；桌面 webview
//     经 Wails AssetServer 加载、无此标记 → owner。标记是「服务时」注入、不入磁盘文件，桌面不受影响。
//   lifecycleRole：D1 桌面为主、非对称 —— 桌面 = owner（关 Tab 即 kill）、浏览器 = viewer
//     （关窗 detach 不 kill；reconcile 的孤儿 kill 在 viewer 不跑，见 TerminalContext）。
import type { EventTransport, HttpClient, TransportKind } from './types';
import { wailsEventTransport } from './wails';
import { httpClient as browserHttpClient, wsEventTransport } from './http';

export type { EventTransport, HttpClient, TransportKind };

declare global {
  interface Window {
    __XKPANE_WEB__?: boolean;
  }
}

// 网关注入了标记 → Web（viewer）；否则桌面 webview（owner）。默认（标记缺失）= 桌面，
// 因为只有经 webapi 网关服务的 HTML 才带标记，浏览器恒会拿到它。
export const isDesktop: boolean = !(typeof window !== 'undefined' && window.__XKPANE_WEB__ === true);

export const transportKind: TransportKind = isDesktop ? 'wails' : 'http';

export type LifecycleRole = 'owner' | 'viewer';
export const lifecycleRole: LifecycleRole = isDesktop ? 'owner' : 'viewer';

// 事件订阅：桌面走 Wails 总线、浏览器走 /ws 镜像。
export const eventTransport: EventTransport = isDesktop ? wailsEventTransport : wsEventTransport;

// httpClient：仅浏览器使用；桌面误用即抛错（RPC 必须走 bindings，bindings 是类型安全的）。
const desktopHttpStub: HttpClient = {
  get: () => Promise.reject(new Error('transport: httpClient 不应在桌面端使用（RPC 走 bindings）')),
  post: () => Promise.reject(new Error('transport: httpClient 不应在桌面端使用（RPC 走 bindings）')),
};
export const httpClient: HttpClient = isDesktop ? desktopHttpStub : browserHttpClient;
