// 浏览器 transport 实现（P7 M2）：
//   - WsEventClient：单条 /ws 长连接，按事件名本地扇出；断线指数退避重连（重连后后端续推
//     live 事件，attach 历史重放是 M4）。httpOnly cookie 在 WS 握手时同源自动携带，token 不进 JS。
//   - httpClient：同源 fetch（credentials:'include' 带 cookie）。失败抛 Error（带后端 {error} 文案）。
import type { EventTransport, HttpClient } from './types';

type Handler = (data: unknown) => void;

const RECONNECT_MIN_MS = 1000;
const RECONNECT_MAX_MS = 30000;

class WsEventClient implements EventTransport {
  private readonly handlers = new Map<string, Set<Handler>>();
  private ws: WebSocket | null = null;
  private connecting = false;
  private reconnectMs = RECONNECT_MIN_MS;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  on<T>(event: string, handler: (data: T) => void): () => void {
    let set = this.handlers.get(event);
    if (!set) {
      set = new Set();
      this.handlers.set(event, set);
    }
    set.add(handler as Handler);
    this.ensureConnected();
    return () => {
      this.handlers.get(event)?.delete(handler as Handler);
      // 连接保持常开（成本低，且 TerminalProvider 的 onLaunch 订阅长驻）。
    };
  }

  private ensureConnected(): void {
    if (this.ws || this.connecting || typeof WebSocket === 'undefined') return;
    this.connect();
  }

  private connect(): void {
    this.connecting = true;
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    let ws: WebSocket;
    try {
      ws = new WebSocket(`${proto}://${location.host}/ws`);
    } catch {
      this.connecting = false;
      this.scheduleReconnect();
      return;
    }
    this.ws = ws;
    ws.onopen = () => {
      this.connecting = false;
      this.reconnectMs = RECONNECT_MIN_MS; // 连上即重置退避
    };
    ws.onmessage = (ev: MessageEvent) => {
      let env: { event?: unknown; data?: unknown };
      try {
        env = JSON.parse(typeof ev.data === 'string' ? ev.data : '');
      } catch {
        return;
      }
      if (!env || typeof env.event !== 'string') return;
      const set = this.handlers.get(env.event);
      if (!set) return;
      for (const h of set) {
        try {
          h(env.data);
        } catch {
          // 单个 handler 抛错不影响其余扇出。
        }
      }
    };
    ws.onclose = () => {
      this.ws = null;
      this.connecting = false;
      this.scheduleReconnect();
    };
    ws.onerror = () => {
      // onerror 后浏览器会接着触发 onclose（在那里统一重连）。主动关闭确保走到 onclose。
      try {
        ws.close();
      } catch {
        /* ignore */
      }
    };
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) return;
    // 无人订阅则不重连（避免空连接）。TerminalProvider 的 onLaunch 长驻 → Web 端实际恒有订阅。
    if (this.handlers.size === 0) return;
    const delay = this.reconnectMs;
    this.reconnectMs = Math.min(this.reconnectMs * 2, RECONNECT_MAX_MS);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.ensureConnected();
    }, delay);
  }
}

export const wsEventTransport: EventTransport = new WsEventClient();

async function request<T>(method: 'GET' | 'POST', path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, credentials: 'include' };
  if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' };
    init.body = JSON.stringify(body);
  }
  const res = await fetch(path, init);
  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      const j = (await res.json()) as { error?: string };
      if (j?.error) msg = j.error;
    } catch {
      /* 非 JSON 错误体，沿用状态码文案 */
    }
    throw new Error(msg);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const httpClient: HttpClient = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
};
