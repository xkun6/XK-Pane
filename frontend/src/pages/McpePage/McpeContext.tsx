// McpeContext — MC PE 页面级状态容器（对应 cc-pane web/stores/useMCPEStore.ts，项目已弃 zustand 用 Context）。
//
// 持有 packages / activePackageId / runs（key=runId），封装全部 action，并订阅后端三事件
// （mc_pe:output 合批 50ms + 2000 行截断 / mc_pe:exited 打标记保留日志 / mc_pe:launched 占位）。
// 仅在桌面挂载（MC PE 纯 Windows）；页面 keepAlive=true 故首次进入后状态/订阅常驻，离开不丢日志。
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
  MCPEService,
  type MCPackage,
  type MCRunSessionInfo,
  type MCOutputPayload,
  type MCExitedPayload,
} from '@/services';

// 合批与截断魔法值（忠实 useMCPEStore.ts）。
const FLUSH_INTERVAL_MS = 50; // 日志合批间隔（~20Hz）
const MAX_OUTPUT_LINES = 2000; // 每 run 最大保留行数
const RESTART_TIMEOUT_MS = 5000; // restart 等 exited 超时

export interface MCOutputLine {
  text: string;
  severity: string; // error|warn|success|info|verbose|developer|plain
  timestamp?: string | null;
  category?: string | null;
  level?: string | null;
}

export interface MCRunRuntime {
  info: MCRunSessionInfo;
  output: MCOutputLine[];
  exited?: { code: number };
}

export interface McpeContextValue {
  packages: MCPackage[];
  activePackageId: string | null;
  runs: Record<string, MCRunRuntime>;
  loadPackages: () => Promise<void>;
  setActivePackage: (id: string | null) => void;
  createPackage: (projectPath: string, name?: string) => Promise<MCPackage>;
  updatePackage: (pkg: MCPackage) => Promise<void>;
  deletePackage: (id: string) => Promise<void>;
  launch: (packageId: string) => Promise<void>;
  stop: (packageId: string) => Promise<void>;
  restart: (packageId: string) => Promise<void>;
  moveWorldToTrash: (id: string) => Promise<void>;
  scanLaunchers: () => Promise<string[]>;
}

const McpeContext = createContext<McpeContextValue | null>(null);

function findRunIdByPackage(runs: Record<string, MCRunRuntime>, packageId: string): string | undefined {
  return Object.values(runs).find((r) => r.info.packageId === packageId)?.info.runId;
}

export function McpeProvider({ children }: { children: ReactNode }) {
  const [packages, setPackages] = useState<MCPackage[]>([]);
  const [activePackageId, setActivePackageId] = useState<string | null>(null);
  const [runs, setRuns] = useState<Record<string, MCRunRuntime>>({});

  const aliveRef = useRef(true);
  const runsRef = useRef(runs); // 给事件/restart 读当前 runs，避免闭包过期
  useEffect(() => {
    runsRef.current = runs;
  }, [runs]);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  const loadPackages = useCallback(async () => {
    const list = await MCPEService.list();
    if (aliveRef.current) setPackages(list);
  }, []);

  const setActivePackage = useCallback((id: string | null) => setActivePackageId(id), []);

  const createPackage = useCallback(async (projectPath: string, name?: string): Promise<MCPackage> => {
    const pkg = await MCPEService.create(projectPath, name ?? '');
    if (aliveRef.current) {
      setPackages((prev) => [pkg, ...prev]); // 新包置顶
      setActivePackageId(pkg.id);
    }
    return pkg;
  }, []);

  const updatePackage = useCallback(async (pkg: MCPackage) => {
    await MCPEService.update(pkg);
    if (aliveRef.current) setPackages((prev) => prev.map((p) => (p.id === pkg.id ? pkg : p)));
  }, []);

  const deletePackage = useCallback(async (id: string) => {
    await MCPEService.remove(id);
    if (!aliveRef.current) return;
    setPackages((prev) => prev.filter((p) => p.id !== id));
    setActivePackageId((cur) => (cur === id ? null : cur));
  }, []);

  const launch = useCallback(async (packageId: string) => {
    const info = await MCPEService.launch(packageId);
    if (!aliveRef.current) return;
    setRuns((prev) => {
      // 清理同包旧 run（含上次 exited 日志）——「重启清空日志」发生在 launch 而非 exited。
      const next: Record<string, MCRunRuntime> = {};
      for (const [rid, rt] of Object.entries(prev)) {
        if (rt.info.packageId !== packageId) next[rid] = rt;
      }
      next[info.runId] = { info, output: [] };
      return next;
    });
  }, []);

  const stop = useCallback(async (packageId: string) => {
    const runId = findRunIdByPackage(runsRef.current, packageId);
    if (!runId) return;
    await MCPEService.stop(runId); // 只 kill；退出由 mc_pe:exited 打标记
  }, []);

  const restart = useCallback(
    async (packageId: string) => {
      const runId = findRunIdByPackage(runsRef.current, packageId);
      if (!runId || runsRef.current[runId]?.exited) {
        await launch(packageId);
        return;
      }
      const exitedP = new Promise<void>((resolve) => {
        let timer: ReturnType<typeof setTimeout>;
        const off = MCPEService.onExited((p: MCExitedPayload) => {
          if (p.runId === runId) {
            off();
            clearTimeout(timer);
            resolve();
          }
        });
        timer = setTimeout(() => {
          off();
          resolve();
        }, RESTART_TIMEOUT_MS); // MC 偶尔不响应 kill，超时兜底
      });
      await MCPEService.stop(runId);
      await exitedP;
      await launch(packageId);
    },
    [launch],
  );

  const moveWorldToTrash = useCallback(async (id: string) => {
    await MCPEService.moveWorldToTrash(id);
  }, []);

  const scanLaunchers = useCallback(() => MCPEService.scanLaunchers(), []);

  // 事件订阅（挂一次）：output 合批，exited 打标记。
  useEffect(() => {
    const buffers = new Map<string, MCOutputLine[]>();
    let flushTimer: ReturnType<typeof setTimeout> | null = null;

    const flush = () => {
      flushTimer = null;
      if (buffers.size === 0) return;
      const batch = new Map(buffers);
      buffers.clear();
      setRuns((prev) => {
        let dirty = false;
        const next = { ...prev };
        for (const [runId, lines] of batch) {
          const rt = next[runId];
          if (!rt) continue;
          const merged = rt.output.concat(lines);
          const trimmed = merged.length > MAX_OUTPUT_LINES ? merged.slice(merged.length - MAX_OUTPUT_LINES) : merged;
          next[runId] = { ...rt, output: trimmed };
          dirty = true;
        }
        return dirty ? next : prev;
      });
    };

    const offOutput = MCPEService.onOutput((p: MCOutputPayload) => {
      const line: MCOutputLine = {
        text: p.line,
        severity: p.severity || 'plain', // 缺省 plain
        timestamp: p.timestamp,
        category: p.category,
        level: p.level,
      };
      const buf = buffers.get(p.runId);
      if (buf) buf.push(line);
      else buffers.set(p.runId, [line]);
      if (flushTimer === null) flushTimer = setTimeout(flush, FLUSH_INTERVAL_MS);
    });

    const offExited = MCPEService.onExited((p: MCExitedPayload) => {
      setRuns((prev) => {
        const rt = prev[p.runId];
        if (!rt) return prev;
        return { ...prev, [p.runId]: { ...rt, exited: { code: p.code } } };
      });
    });

    return () => {
      offOutput();
      offExited();
      if (flushTimer !== null) clearTimeout(flushTimer);
    };
  }, []);

  const value = useMemo<McpeContextValue>(
    () => ({
      packages,
      activePackageId,
      runs,
      loadPackages,
      setActivePackage,
      createPackage,
      updatePackage,
      deletePackage,
      launch,
      stop,
      restart,
      moveWorldToTrash,
      scanLaunchers,
    }),
    [
      packages,
      activePackageId,
      runs,
      loadPackages,
      setActivePackage,
      createPackage,
      updatePackage,
      deletePackage,
      launch,
      stop,
      restart,
      moveWorldToTrash,
      scanLaunchers,
    ],
  );

  return <McpeContext.Provider value={value}>{children}</McpeContext.Provider>;
}

export function useMcpe(): McpeContextValue {
  const ctx = useContext(McpeContext);
  if (!ctx) throw new Error('useMcpe must be used within <McpeProvider>');
  return ctx;
}

// isRunning 派生：该包存在一个未退出的 run。
export function isPackageRunning(runs: Record<string, MCRunRuntime>, packageId: string): boolean {
  return Object.values(runs).some((r) => r.info.packageId === packageId && !r.exited);
}
