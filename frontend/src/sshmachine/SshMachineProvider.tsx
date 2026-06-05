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
import {
  SshMachineService,
  type SshMachine,
  type SshMachineInput,
  type SshConnectivityResult,
} from '@/services';
import type { SshMachineContextValue } from './types';

const SshMachineContext = createContext<SshMachineContextValue | null>(null);

// SSH 机器全局 Provider（SQLite 后端）。范式同 ProviderProvider：首屏空列表立即渲染，
// mount 异步拉取；mutation 调后端后 reload 刷新。错误不吞——向上抛给调用方（View 用
// NativeDialogs 提示）。被 sshmachine 设置子页与（后续）启动选机器 UI 共同消费。
export interface SshMachineProviderProps {
  children: ReactNode;
}

export const SshMachineProvider = ({ children }: SshMachineProviderProps) => {
  const [machines, setMachines] = useState<SshMachine[]>([]);
  const [loading, setLoading] = useState(true);

  const aliveRef = useRef(true);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  const reload = useCallback(async (): Promise<void> => {
    const rows = await SshMachineService.list();
    if (aliveRef.current) setMachines(rows);
  }, []);

  useEffect(() => {
    let cancelled = false;
    SshMachineService.list()
      .then((rows) => {
        if (!cancelled && aliveRef.current) setMachines(rows);
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

  const addMachine = useCallback(
    async (input: SshMachineInput): Promise<SshMachine> => {
      const created = await SshMachineService.add(input);
      await reload();
      return created;
    },
    [reload],
  );

  const updateMachine = useCallback(
    async (id: string, input: SshMachineInput): Promise<void> => {
      await SshMachineService.update(id, input);
      await reload();
    },
    [reload],
  );

  const removeMachine = useCallback(
    async (id: string): Promise<void> => {
      await SshMachineService.remove(id);
      await reload();
    },
    [reload],
  );

  // 连通性测试不改状态，直接透传后端结果（View 据此提示）。
  const checkConnectivity = useCallback(
    (id: string): Promise<SshConnectivityResult> => SshMachineService.checkConnectivity(id),
    [],
  );

  const value = useMemo<SshMachineContextValue>(
    () => ({ machines, loading, refresh, addMachine, updateMachine, removeMachine, checkConnectivity }),
    [machines, loading, refresh, addMachine, updateMachine, removeMachine, checkConnectivity],
  );

  return <SshMachineContext.Provider value={value}>{children}</SshMachineContext.Provider>;
};

export const useSshMachines = (): SshMachineContextValue => {
  const ctx = useContext(SshMachineContext);
  if (!ctx) {
    throw new Error('useSshMachines must be used inside <SshMachineProvider>');
  }
  return ctx;
};
