import { useCallback, useEffect, useRef, useState } from 'react';
import { BotBridgeService, type BotBridgeStatus } from '@/services';

export type BotbridgeSaveStatus =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'saving' }
  | { kind: 'saved' }
  | { kind: 'failed'; message: string };

export interface UseBotbridgeResult {
  platform: string;
  appId: string;
  appSecret: string;
  enabled: boolean;
  connected: boolean;
  error: string;
  status: BotbridgeSaveStatus;
  setPlatform: (value: string) => void;
  setAppId: (value: string) => void;
  setAppSecret: (value: string) => void;
  setEnabled: (value: boolean) => void;
  refresh: () => Promise<void>;
  save: () => Promise<void>;
}

export const useBotbridge = (): UseBotbridgeResult => {
  const [platform, setPlatform] = useState('feishu');
  const [appId, setAppId] = useState('');
  const [appSecret, setAppSecret] = useState('');
  const [enabled, setEnabled] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState('');
  const [status, setStatus] = useState<BotbridgeSaveStatus>({ kind: 'loading' });
  const aliveRef = useRef(true);

  const applyStatus = useCallback((s: BotBridgeStatus) => {
    setPlatform(s.platform || 'feishu');
    setAppId(s.appId || '');
    setEnabled(s.enabled);
    setConnected(s.connected);
    setError(s.error || '');
  }, []);

  const refresh = useCallback(async () => {
    setStatus({ kind: 'loading' });
    try {
      const s = await BotBridgeService.getStatus();
      if (!aliveRef.current) return;
      applyStatus(s);
      setStatus({ kind: 'idle' });
    } catch (err) {
      if (!aliveRef.current) return;
      setStatus({ kind: 'failed', message: messageOf(err) });
    }
  }, [applyStatus]);

  useEffect(() => {
    aliveRef.current = true;
    void refresh();
    return () => {
      aliveRef.current = false;
    };
  }, [refresh]);

  const save = useCallback(async () => {
    setStatus({ kind: 'saving' });
    try {
      await BotBridgeService.setConfig({ platform, appId, appSecret, enabled });
      const s = await BotBridgeService.getStatus();
      if (!aliveRef.current) return;
      applyStatus(s);
      setAppSecret('');
      setStatus({ kind: 'saved' });
    } catch (err) {
      if (!aliveRef.current) return;
      setStatus({ kind: 'failed', message: messageOf(err) });
    }
  }, [appId, appSecret, applyStatus, enabled, platform]);

  return {
    platform,
    appId,
    appSecret,
    enabled,
    connected,
    error,
    status,
    setPlatform,
    setAppId,
    setAppSecret,
    setEnabled,
    refresh,
    save,
  };
};

const messageOf = (err: unknown): string => (err instanceof Error ? err.message : String(err));

