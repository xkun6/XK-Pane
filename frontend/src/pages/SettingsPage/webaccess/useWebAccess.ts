import { useCallback, useEffect, useRef, useState } from 'react';
import { WebAccessService, type AccessInfo } from '@/services';

export type WebAccessStatus =
  | { kind: 'loading' }
  | { kind: 'ready' }
  | { kind: 'unavailable'; message: string };

export interface UseWebAccessResult {
  info: AccessInfo | null;
  status: WebAccessStatus;
  /** 最近一次"已复制"的 URL（用于在对应行显示反馈），2s 后清空。 */
  copiedUrl: string | null;
  refresh: () => Promise<void>;
  copyUrl: (url: string) => Promise<void>;
}

// Web 访问子页面 ViewModel：
//   - mount 拉一次 GetAccessInfo；网关未启动则进入 unavailable（可重试）
//   - refresh 重新拉取（换网卡 / 启动失败后重试）
//   - copyUrl 走 navigator.clipboard（标准 Web API，非 Wails 绑定），失败静默
export const useWebAccess = (): UseWebAccessResult => {
  const [info, setInfo] = useState<AccessInfo | null>(null);
  const [status, setStatus] = useState<WebAccessStatus>({ kind: 'loading' });
  const [copiedUrl, setCopiedUrl] = useState<string | null>(null);
  const aliveRef = useRef(true);

  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  const refresh = useCallback(async (): Promise<void> => {
    setStatus({ kind: 'loading' });
    try {
      const a = await WebAccessService.getAccessInfo();
      if (!aliveRef.current) return;
      setInfo(a);
      setStatus({ kind: 'ready' });
    } catch (err) {
      if (!aliveRef.current) return;
      setStatus({ kind: 'unavailable', message: err instanceof Error ? err.message : String(err) });
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const copyUrl = useCallback(async (url: string): Promise<void> => {
    try {
      await navigator.clipboard?.writeText(url);
      if (!aliveRef.current) return;
      setCopiedUrl(url);
      window.setTimeout(() => {
        if (aliveRef.current) setCopiedUrl(null);
      }, 2000);
    } catch {
      // 剪贴板不可用：静默（用户仍可手动选中复制）
    }
  }, []);

  return { info, status, copiedUrl, refresh, copyUrl };
};
