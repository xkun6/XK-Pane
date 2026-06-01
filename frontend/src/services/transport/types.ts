// transport 抽象契约（P7 §6.1 / M2）。两件事桌面与浏览器实现不同：
//   - 事件订阅：桌面经 @wailsio/runtime 应用总线、浏览器经 /ws 镜像广播。
//   - RPC：桌面经编译期 bindings（数字 MethodID）、浏览器经同源 HTTP（cookie 鉴权）。
// services 层据 isDesktop 选实现；View / ViewModel 全程不感知（铁律：transport 抽象只在 services/）。

export type TransportKind = 'wails' | 'http';

// EventTransport：按事件名订阅后端广播，回调收到的是**解包后的 payload**（与桌面 Wails
// 事件的 e.data 同构）。返回取消函数（务必在 useEffect 里 return）。
export interface EventTransport {
  on<T = unknown>(event: string, handler: (data: T) => void): () => void;
}

// HttpClient：浏览器侧 REST（同源、credentials:'include' 带 httpOnly cookie）。
// 桌面端不用它（RPC 走 bindings）—— 桌面实现是会抛错的占位，防误用。
export interface HttpClient {
  get<T = unknown>(path: string): Promise<T>;
  post<T = unknown>(path: string, body?: unknown): Promise<T>;
}
