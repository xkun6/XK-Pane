// 桌面 transport 实现：事件经 @wailsio/runtime 应用级总线。
// 解包 Wails 事件信封的 .data，使回调收到的与浏览器 /ws 路径同构（都交付原始 payload）。
import { Events } from '@wailsio/runtime';
import type { EventTransport } from './types';

interface WailsEventEnvelope<T> {
  data?: T;
  name?: string;
  sender?: string;
}

export const wailsEventTransport: EventTransport = {
  on<T>(event: string, handler: (data: T) => void): () => void {
    const off = Events.On(event, (e: WailsEventEnvelope<T> | undefined) => {
      handler(e?.data as T);
    });
    return () => {
      if (typeof off === 'function') off();
    };
  },
};
