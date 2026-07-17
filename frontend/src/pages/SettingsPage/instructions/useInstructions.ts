import { useCallback, useEffect, useRef, useState } from 'react';
import { AgentCfgService, NativeDialogs } from '@/services';

// 全局指令子页面的 ViewModel：Claude / Codex 两个 tab 各自懒加载 + 持有草稿，
// 切 tab 不丢草稿；不返回 JSX、不持翻译串——对话框文案由 View（持 t()）传入（MVVM 铁律）。
//
// 首次懒加载失败 → 静默置空串 + loadError 置位，由 View 用 t('settings.instructions.loadError')
// 展示成提示文字（不弹窗打断）；用户主动点「重新加载」失败 → 弹 NativeDialogs.error（strings
// 由 View 传入），因为这是一次显式操作，用户需要明确知道结果。
//
// InstructionApp 刻意不直接复用 AgentAppType（Provider 域的 appType 联合类型）——后端
// ReadInstructionFile/WriteInstructionFile 明确拒绝 grok（全局指令文件位置未经确认，见
// agentcfg/instructions.go），本页也没有 grok 标签，这里锁定成独立的 claude|codex
// 字面量联合，防止 Provider 域将来再扩类型时本页被动跟着"长出"一个打不开的 grok 入口。
export type InstructionApp = 'claude' | 'codex';

interface AppState {
  draft: string;
  saved: string;
  loaded: boolean;
}

const emptyAppState = (): AppState => ({ draft: '', saved: '', loaded: false });

function withAdded<T>(set: ReadonlySet<T>, v: T): ReadonlySet<T> {
  const next = new Set(set);
  next.add(v);
  return next;
}

function withRemoved<T>(set: ReadonlySet<T>, v: T): ReadonlySet<T> {
  if (!set.has(v)) return set;
  const next = new Set(set);
  next.delete(v);
  return next;
}

export interface ErrorStrings {
  errorTitle: string;
}

export interface ReloadStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export interface UseInstructionsResult {
  tab: InstructionApp;
  setTab: (t: InstructionApp) => void;
  content: string; // 当前 tab 的草稿
  dirty: boolean;
  loading: boolean;
  saving: boolean;
  loadError: boolean; // 初次加载失败：VM 不持翻译串，由 View 用 t() 展示成提示文字
  setContent: (v: string) => void;
  save: (strings: ErrorStrings) => Promise<void>;
  reload: (strings: ReloadStrings) => Promise<void>;
}

export const useInstructions = (): UseInstructionsResult => {
  const [tab, setTab] = useState<InstructionApp>('claude');
  const [states, setStates] = useState<Record<InstructionApp, AppState>>({
    claude: emptyAppState(),
    codex: emptyAppState(),
  });
  const [loadingSet, setLoadingSet] = useState<ReadonlySet<InstructionApp>>(new Set());
  const [loadErrorSet, setLoadErrorSet] = useState<ReadonlySet<InstructionApp>>(new Set());
  const [saving, setSaving] = useState(false);

  // 用 ref 拿最新 states，避免 save/reload 的 useCallback 依赖 states 而频繁重建。
  const statesRef = useRef(states);
  statesRef.current = states;

  // 首次懒加载（tab 首次激活触发）：失败静默吞掉，只置 loadError 标志，不弹窗。
  const loadApp = useCallback(async (app: InstructionApp): Promise<void> => {
    setLoadingSet((prev) => withAdded(prev, app));
    try {
      const text = await AgentCfgService.readInstructionFile(app);
      setStates((prev) => ({ ...prev, [app]: { draft: text, saved: text, loaded: true } }));
      setLoadErrorSet((prev) => withRemoved(prev, app));
    } catch {
      setStates((prev) => ({ ...prev, [app]: { ...prev[app], loaded: true } }));
      setLoadErrorSet((prev) => withAdded(prev, app));
    } finally {
      setLoadingSet((prev) => withRemoved(prev, app));
    }
  }, []);

  // 每个 tab 首次激活时懒加载一次，此后切回不重新拉取（草稿留在 states 里）。
  const loadStartedRef = useRef<Set<InstructionApp>>(new Set());
  useEffect(() => {
    if (loadStartedRef.current.has(tab)) return;
    loadStartedRef.current.add(tab);
    void loadApp(tab);
  }, [tab, loadApp]);

  const setContent = useCallback((v: string) => {
    setStates((prev) => ({ ...prev, [tab]: { ...prev[tab], draft: v } }));
  }, [tab]);

  const save = useCallback(
    async (strings: ErrorStrings): Promise<void> => {
      const app = tab;
      const draft = statesRef.current[app].draft;
      setSaving(true);
      try {
        await AgentCfgService.writeInstructionFile(app, draft);
        setStates((prev) => ({ ...prev, [app]: { ...prev[app], saved: draft } }));
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      } finally {
        setSaving(false);
      }
    },
    [tab],
  );

  // 用户主动「重新加载」：dirty 时先确认丢弃草稿；失败弹窗告知（这是显式操作，不能静默）。
  const reload = useCallback(
    async (strings: ReloadStrings): Promise<void> => {
      const app = tab;
      const cur = statesRef.current[app];
      if (cur.draft !== cur.saved) {
        const ok = await NativeDialogs.confirm({
          title: strings.confirmTitle,
          message: strings.confirmMessage,
          okLabel: strings.okLabel,
          cancelLabel: strings.cancelLabel,
        });
        if (!ok) return;
      }
      setLoadingSet((prev) => withAdded(prev, app));
      try {
        const text = await AgentCfgService.readInstructionFile(app);
        setStates((prev) => ({ ...prev, [app]: { draft: text, saved: text, loaded: true } }));
        setLoadErrorSet((prev) => withRemoved(prev, app));
      } catch (e) {
        await NativeDialogs.error(strings.errorTitle, messageOf(e));
      } finally {
        setLoadingSet((prev) => withRemoved(prev, app));
      }
    },
    [tab],
  );

  const current = states[tab];
  return {
    tab,
    setTab,
    content: current.draft,
    dirty: current.draft !== current.saved,
    loading: loadingSet.has(tab),
    saving,
    loadError: loadErrorSet.has(tab),
    setContent,
    save,
    reload,
  };
};
