import { useCallback, useEffect, useRef, useState } from 'react';
import {
  BedrockDocSyncService,
  type SyncStatus,
  type SyncReport,
  type BedrockDocSyncProgress,
  type DocSyncSource,
} from '@/services';

// 知识库子页面 ViewModel：OTA 文档同步的状态 / 触发 / 取消 / 重置 + 代理 / 源配置持久化。
// 进度事件订阅在 mount 期建立，sync() 期间 live 更新 progress / logLines。

// 一条原始日志（来自 stage="log" 事件，verbatim 展示，类比终端输出不走 t()）。
export interface LogLine {
  level: string; // info | warn | error
  msg: string;
}

// 代理表单（受控）。
export interface ProxyForm {
  url: string;
  auth: string;
}

// 源覆盖表单（受控；空 = 用官方默认）。
export interface SourcesForm {
  mcguideRemote: string;
  mcguideBranch: string;
  mconlineRemote: string;
  mconlineBranch: string;
  mcdocsBaseUrl: string;
}

// 动作反馈（reset / proxy / sources 操作的瞬时状态，供视图渲染提示）。
export type ActionStatus =
  | { kind: 'idle' }
  | { kind: 'resetting' }
  | { kind: 'reset' }
  | { kind: 'savingSources' }
  | { kind: 'savedSources' }
  | { kind: 'testingProxy' }
  | { kind: 'proxyOk'; latencyMs: number }
  | { kind: 'proxyFail' }
  | { kind: 'failed'; message: string };

export const ALL_SOURCES: DocSyncSource[] = ['mcguide', 'mconline', 'mcdocs'];
const DEFAULT_SOURCES: DocSyncSource[] = ['mcguide', 'mconline'];
const LOG_CAP = 200;

const emptySourcesForm = (): SourcesForm => ({
  mcguideRemote: '',
  mcguideBranch: '',
  mconlineRemote: '',
  mconlineBranch: '',
  mcdocsBaseUrl: '',
});

export interface UseKnowledgeResult {
  status: SyncStatus | null;
  syncing: boolean;
  progress: BedrockDocSyncProgress | null;
  logLines: LogLine[];
  lastReport: SyncReport | null;
  selectedSources: DocSyncSource[];
  proxyForm: ProxyForm;
  sourcesForm: SourcesForm;
  action: ActionStatus;
  refresh: () => Promise<void>;
  toggleSource: (s: DocSyncSource) => void;
  runSync: () => Promise<void>;
  cancelSync: () => Promise<void>;
  resetData: () => Promise<void>;
  setProxyField: (field: keyof ProxyForm, value: string) => void;
  testAndSaveProxy: () => Promise<void>;
  setSourcesField: (field: keyof SourcesForm, value: string) => void;
  saveSources: () => Promise<void>;
}

export const useKnowledge = (): UseKnowledgeResult => {
  const [status, setStatus] = useState<SyncStatus | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [progress, setProgress] = useState<BedrockDocSyncProgress | null>(null);
  const [logLines, setLogLines] = useState<LogLine[]>([]);
  const [lastReport, setLastReport] = useState<SyncReport | null>(null);
  const [selectedSources, setSelectedSources] = useState<DocSyncSource[]>(DEFAULT_SOURCES);
  const [proxyForm, setProxyForm] = useState<ProxyForm>({ url: '', auth: '' });
  const [sourcesForm, setSourcesForm] = useState<SourcesForm>(emptySourcesForm);
  const [action, setAction] = useState<ActionStatus>({ kind: 'idle' });
  const aliveRef = useRef(true);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const s = await BedrockDocSyncService.status();
      if (aliveRef.current) setStatus(s);
    } catch {
      // 静默：保留上次成功值（如 web 端 reject）
    }
  }, []);

  // mount：拉状态 + 预填代理 / 源表单 + 订阅进度事件。
  useEffect(() => {
    aliveRef.current = true;
    void refresh();
    void (async () => {
      try {
        const p = await BedrockDocSyncService.getProxy();
        if (aliveRef.current) setProxyForm({ url: p.httpsProxy ?? '', auth: p.auth ?? '' });
      } catch {
        /* noop */
      }
      try {
        const c = await BedrockDocSyncService.getSources();
        if (aliveRef.current) {
          setSourcesForm({
            mcguideRemote: c.mcguideRemote ?? '',
            mcguideBranch: c.mcguideBranch ?? '',
            mconlineRemote: c.mconlineRemote ?? '',
            mconlineBranch: c.mconlineBranch ?? '',
            mcdocsBaseUrl: c.mcdocsBaseUrl ?? '',
          });
        }
      } catch {
        /* noop */
      }
    })();

    const off = BedrockDocSyncService.onProgress((evt) => {
      if (!aliveRef.current) return;
      if (evt.stage === 'log') {
        const line: LogLine = { level: evt.level ?? 'info', msg: evt.msg ?? '' };
        setLogLines((prev) => {
          const next = prev.length >= LOG_CAP ? prev.slice(prev.length - LOG_CAP + 1) : prev.slice();
          next.push(line);
          return next;
        });
        return;
      }
      setProgress(evt);
    });
    return () => {
      aliveRef.current = false;
      off();
    };
  }, [refresh]);

  const toggleSource = useCallback((s: DocSyncSource): void => {
    setSelectedSources((prev) =>
      prev.includes(s) ? prev.filter((x) => x !== s) : [...ALL_SOURCES.filter((x) => x === s || prev.includes(x))],
    );
  }, []);

  const runSync = useCallback(async (): Promise<void> => {
    if (selectedSources.length === 0) return;
    setSyncing(true);
    setProgress(null);
    setLogLines([]);
    setLastReport(null);
    setAction({ kind: 'idle' });
    try {
      const report = await BedrockDocSyncService.sync({ sources: selectedSources });
      if (aliveRef.current) setLastReport(report);
    } catch (err) {
      if (aliveRef.current) setAction({ kind: 'failed', message: getErrorMessage(err) });
    } finally {
      if (aliveRef.current) {
        setSyncing(false);
        void refresh();
      }
    }
  }, [selectedSources, refresh]);

  const cancelSync = useCallback(async (): Promise<void> => {
    try {
      await BedrockDocSyncService.cancel();
    } catch {
      /* noop：取消失败不致命 */
    }
  }, []);

  const resetData = useCallback(async (): Promise<void> => {
    setAction({ kind: 'resetting' });
    try {
      await BedrockDocSyncService.reset();
      await refresh();
      if (aliveRef.current) setAction({ kind: 'reset' });
    } catch (err) {
      if (aliveRef.current) setAction({ kind: 'failed', message: getErrorMessage(err) });
    }
  }, [refresh]);

  const setProxyField = useCallback((field: keyof ProxyForm, value: string): void => {
    setProxyForm((prev) => ({ ...prev, [field]: value }));
  }, []);

  const testAndSaveProxy = useCallback(async (): Promise<void> => {
    setAction({ kind: 'testingProxy' });
    try {
      const probe = await BedrockDocSyncService.setProxy({
        httpsProxy: proxyForm.url.trim(),
        auth: proxyForm.auth.trim(),
      });
      if (!aliveRef.current) return;
      setAction(probe.ok ? { kind: 'proxyOk', latencyMs: probe.latencyMs } : { kind: 'proxyFail' });
    } catch (err) {
      if (aliveRef.current) setAction({ kind: 'failed', message: getErrorMessage(err) });
    }
  }, [proxyForm]);

  const setSourcesField = useCallback((field: keyof SourcesForm, value: string): void => {
    setSourcesForm((prev) => ({ ...prev, [field]: value }));
  }, []);

  const saveSources = useCallback(async (): Promise<void> => {
    setAction({ kind: 'savingSources' });
    try {
      await BedrockDocSyncService.setSources({
        mcguideRemote: sourcesForm.mcguideRemote.trim(),
        mcguideBranch: sourcesForm.mcguideBranch.trim(),
        mconlineRemote: sourcesForm.mconlineRemote.trim(),
        mconlineBranch: sourcesForm.mconlineBranch.trim(),
        mcdocsBaseUrl: sourcesForm.mcdocsBaseUrl.trim(),
      });
      if (aliveRef.current) setAction({ kind: 'savedSources' });
    } catch (err) {
      if (aliveRef.current) setAction({ kind: 'failed', message: getErrorMessage(err) });
    }
  }, [sourcesForm]);

  return {
    status,
    syncing,
    progress,
    logLines,
    lastReport,
    selectedSources,
    proxyForm,
    sourcesForm,
    action,
    refresh,
    toggleSource,
    runSync,
    cancelSync,
    resetData,
    setProxyField,
    testAndSaveProxy,
    setSourcesField,
    saveSources,
  };
};

const getErrorMessage = (err: unknown): string => {
  if (err instanceof Error) return err.message;
  return String(err);
};
