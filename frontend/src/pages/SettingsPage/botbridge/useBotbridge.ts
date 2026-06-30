import { useCallback, useEffect, useRef, useState } from 'react';
import { BotBridgeService, type BotBridgePlatformSpec, type BotBridgeStatus } from '@/services';

export type BotbridgeSaveStatus =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'saving' }
  | { kind: 'saved' }
  | { kind: 'failed'; message: string };

export interface BotbridgeFieldView {
  key: string;
  secret: boolean;
  value: string;
}

export interface UseBotbridgeResult {
  platforms: string[];
  platform: string;
  fields: BotbridgeFieldView[];
  enabled: boolean;
  connected: boolean;
  error: string;
  status: BotbridgeSaveStatus;
  setPlatform: (value: string) => void;
  setField: (key: string, value: string) => void;
  setEnabled: (value: boolean) => void;
  refresh: () => Promise<void>;
  save: () => Promise<void>;
}

const specOf = (specs: BotBridgePlatformSpec[], id: string): BotBridgePlatformSpec | undefined =>
  specs.find((s) => s.id === id);

export const useBotbridge = (): UseBotbridgeResult => {
  const [specs, setSpecs] = useState<BotBridgePlatformSpec[]>([]);
  const [platform, setPlatformState] = useState('feishu');
  const [values, setValues] = useState<Record<string, string>>({});
  const [enabled, setEnabled] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState('');
  const [status, setStatus] = useState<BotbridgeSaveStatus>({ kind: 'loading' });
  const aliveRef = useRef(true);
  // 已保存的非敏感凭据值 + 它们属于哪个平台；切回该平台时回填（敏感字段始终留空）。
  const savedCredsRef = useRef<Record<string, string>>({});
  const savedPlatformRef = useRef('');

  const applyStatus = useCallback((s: BotBridgeStatus) => {
    const next = s.platform || 'feishu';
    setPlatformState(next);
    savedCredsRef.current = s.credentials || {};
    savedPlatformRef.current = next;
    setValues(s.credentials || {});
    setEnabled(s.enabled);
    setConnected(s.connected);
    setError(s.error || '');
  }, []);

  const refresh = useCallback(async () => {
    setStatus({ kind: 'loading' });
    try {
      const [list, s] = await Promise.all([BotBridgeService.listPlatforms(), BotBridgeService.getStatus()]);
      if (!aliveRef.current) return;
      setSpecs(list);
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

  // 连接状态是异步握手（WS OnReady 在 Start 返回后才触发），保存时一次性读取会读到尚未连上的快照。
  // 轮询只刷新 connected / error 这两个「实时态」字段，绝不回写正在编辑的表单（platform/凭据/enabled）。
  useEffect(() => {
    const timer = window.setInterval(() => {
      void (async () => {
        try {
          const s = await BotBridgeService.getStatus();
          if (!aliveRef.current) return;
          setConnected(s.connected);
          setError(s.error || '');
        } catch {
          // 瞬时读取失败忽略，下一拍重试
        }
      })();
    }, 2000);
    return () => window.clearInterval(timer);
  }, []);

  const setPlatform = useCallback((next: string) => {
    setPlatformState(next);
    // 切回已配置平台 → 回填已保存的非敏感值；切到别的平台 → 清空待重新填写。
    setValues(next === savedPlatformRef.current ? { ...savedCredsRef.current } : {});
  }, []);

  const setField = useCallback((key: string, value: string) => {
    setValues((prev) => ({ ...prev, [key]: value }));
  }, []);

  const save = useCallback(async () => {
    setStatus({ kind: 'saving' });
    try {
      const spec = specOf(specs, platform);
      const credentials: Record<string, string> = {};
      for (const f of spec?.fields ?? []) {
        const v = (values[f.key] ?? '').trim();
        if (v !== '') credentials[f.key] = v;
      }
      await BotBridgeService.setConfig({ platform, credentials, enabled });
      const s = await BotBridgeService.getStatus();
      if (!aliveRef.current) return;
      applyStatus(s);
      setStatus({ kind: 'saved' });
    } catch (err) {
      if (!aliveRef.current) return;
      setStatus({ kind: 'failed', message: messageOf(err) });
    }
  }, [applyStatus, enabled, platform, specs, values]);

  const spec = specOf(specs, platform);
  const fields: BotbridgeFieldView[] = (spec?.fields ?? []).map((f) => ({
    key: f.key,
    secret: f.secret,
    value: values[f.key] ?? '',
  }));
  const platforms = specs.map((s) => s.id);

  return {
    platforms,
    platform,
    fields,
    enabled,
    connected,
    error,
    status,
    setPlatform,
    setField,
    setEnabled,
    refresh,
    save,
  };
};

const messageOf = (err: unknown): string => (err instanceof Error ? err.message : String(err));
