import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { ProviderService, type Provider, type ProviderInput } from '@/services';
import type { ProviderContextValue } from './types';

const ProviderContext = createContext<ProviderContextValue | null>(null);

// 接入 Provider 全局 Provider（SQLite 后端，弃 Zustand）。范式同 ProjectProvider：
// 首屏空列表立即渲染，mount 异步拉取；mutation 调后端后 reload 刷新。错误不吞——
// 向上抛给调用方（View 用 NativeDialogs 提示）。被 provider 设置子页与（后续）启动选
// Provider UI 共同消费。
export interface ProviderProviderProps {
  children: ReactNode;
}

export const ProviderProvider = ({ children }: ProviderProviderProps) => {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);

  const aliveRef = useRef(true);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  // 拉取列表（插入序）。供 mount 初始化与每次 mutation 后刷新复用；不切 loading。
  const reload = useCallback(async (): Promise<void> => {
    const rows = await ProviderService.list();
    if (aliveRef.current) setProviders(rows);
  }, []);

  useEffect(() => {
    let cancelled = false;
    ProviderService.list()
      .then((rows) => {
        if (!cancelled && aliveRef.current) setProviders(rows);
      })
      .catch(() => {
        // 后端未就绪 / 异常 —— 保持空列表
      })
      .finally(() => {
        if (!cancelled && aliveRef.current) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const refresh = useCallback((): Promise<void> => reload(), [reload]);

  const addProvider = useCallback(
    async (input: ProviderInput): Promise<Provider> => {
      const created = await ProviderService.add(input);
      await reload();
      return created;
    },
    [reload],
  );

  const updateProvider = useCallback(
    async (id: string, input: ProviderInput): Promise<void> => {
      await ProviderService.update(id, input);
      await reload();
    },
    [reload],
  );

  const removeProvider = useCallback(
    async (id: string): Promise<void> => {
      await ProviderService.remove(id);
      await reload();
    },
    [reload],
  );

  const setDefaultProvider = useCallback(
    async (id: string): Promise<void> => {
      await ProviderService.setDefault(id);
      await reload();
    },
    [reload],
  );

  const value = useMemo<ProviderContextValue>(
    () => ({ providers, loading, refresh, addProvider, updateProvider, removeProvider, setDefaultProvider }),
    [providers, loading, refresh, addProvider, updateProvider, removeProvider, setDefaultProvider],
  );

  return <ProviderContext.Provider value={value}>{children}</ProviderContext.Provider>;
};

export const useProviders = (): ProviderContextValue => {
  const ctx = useContext(ProviderContext);
  if (!ctx) {
    throw new Error('useProviders must be used inside <ProviderProvider>');
  }
  return ctx;
};
