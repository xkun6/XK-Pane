import { useCallback, useEffect, useRef, useState } from 'react';
import {
  PreferencesService,
  ScreenshotService,
  SCREENSHOT_DEFAULT_RETENTION_DAYS,
  SCREENSHOT_DEFAULT_SHORTCUT,
  SCREENSHOT_PREF_RETENTION_DAYS,
  SCREENSHOT_PREF_SHORTCUT,
} from '@/services';
import { comboFromKeyboardEvent, type ComboKeySource } from './shortcutCombo';

export type ScreenshotStatus =
  | { kind: 'idle' }
  | { kind: 'applying' }
  | { kind: 'applied' }
  | { kind: 'failed'; message: string };

/** 录制框 keydown 事件的最小结构（React.KeyboardEvent 兼容）。 */
export type RecordKeyEvent = ComboKeySource & {
  preventDefault: () => void;
  currentTarget: { blur: () => void };
};

export interface UseScreenshotResult {
  /** 已生效（持久化）的快捷键。 */
  shortcut: string;
  /** 录制中待应用的新组合；null = 未录到。 */
  pending: string | null;
  recording: boolean;
  /** 保留天数输入草稿（字符串，允许编辑中的空态）；失焦时 commitRetention 解析落库。 */
  retentionDraft: string;
  status: ScreenshotStatus;
  startRecording: () => void;
  stopRecording: () => void;
  /** 录制框 keydown：拼组合串（Escape = 退出录制并移走焦点；Tab 放行防键盘陷阱）。 */
  onRecordKey: (e: RecordKeyEvent) => void;
  /** 应用 pending 组合：后端重注册成功后才落库；失败时回滚注册旧键。 */
  applyShortcut: () => Promise<void>;
  cancelPending: () => void;
  setRetentionDraft: (value: string) => void;
  commitRetention: () => void;
}

// 截图设置子页 ViewModel：
//   - mount 从 preferences 拉快捷键 + 保留天数（默认值兜底首屏）
//   - 录制：keydown → comboFromKeyboardEvent 拼串（参照 cc-pane ScreenshotSection）
//   - 应用：ScreenshotService.updateShortcut（后端重注册）成功 → PreferencesService 落库；
//     失败时旧键已被后端注销 → 回滚注册旧键，保持 UI 与真实注册态一致
//   - 保留天数：草稿态编辑、失焦提交（避免逐键持久化与清空瞬间写 0）
export const useScreenshot = (): UseScreenshotResult => {
  const [shortcut, setShortcut] = useState<string>(SCREENSHOT_DEFAULT_SHORTCUT);
  const [pending, setPending] = useState<string | null>(null);
  const [recording, setRecording] = useState(false);
  const [retentionDays, setRetentionDays] = useState<number>(SCREENSHOT_DEFAULT_RETENTION_DAYS);
  const [retentionDraft, setRetentionDraft] = useState<string>(
    String(SCREENSHOT_DEFAULT_RETENTION_DAYS),
  );
  const [status, setStatus] = useState<ScreenshotStatus>({ kind: 'idle' });
  const aliveRef = useRef(true);

  useEffect(() => {
    aliveRef.current = true;
    void (async () => {
      const [s, r] = await Promise.allSettled([
        PreferencesService.get<string>(SCREENSHOT_PREF_SHORTCUT),
        PreferencesService.get<number>(SCREENSHOT_PREF_RETENTION_DAYS),
      ]);
      if (!aliveRef.current) return;
      if (s.status === 'fulfilled' && typeof s.value === 'string' && s.value.trim() !== '') {
        setShortcut(s.value);
      }
      if (r.status === 'fulfilled' && typeof r.value === 'number' && r.value >= 0) {
        setRetentionDays(Math.floor(r.value));
        setRetentionDraft(String(Math.floor(r.value)));
      }
    })();
    return () => {
      aliveRef.current = false;
    };
  }, []);

  const startRecording = useCallback(() => {
    setRecording(true);
    setStatus({ kind: 'idle' });
  }, []);

  const stopRecording = useCallback(() => setRecording(false), []);

  const onRecordKey = useCallback(
    (e: RecordKeyEvent) => {
      if (e.key === 'Tab') return; // 放行焦点迁移（防键盘陷阱，「应用」按钮要靠 Tab 到达）
      if (!recording) return;
      e.preventDefault();
      if (e.key === 'Escape') {
        setPending(null);
        setRecording(false);
        e.currentTarget.blur(); // 焦点随视觉状态一起离开录制框
        return;
      }
      const combo = comboFromKeyboardEvent(e);
      if (combo) setPending(combo);
    },
    [recording],
  );

  const applyShortcut = useCallback(async (): Promise<void> => {
    if (!pending || pending === shortcut) {
      setPending(null);
      setRecording(false);
      return;
    }
    setStatus({ kind: 'applying' });
    try {
      await ScreenshotService.updateShortcut(pending);
      await PreferencesService.set(SCREENSHOT_PREF_SHORTCUT, pending);
      if (!aliveRef.current) return;
      setShortcut(pending);
      setPending(null);
      setRecording(false);
      setStatus({ kind: 'applied' });
    } catch (err) {
      // 后端语义：失败时旧热键已被注销 → 回滚注册旧键，避免「界面显示旧键生效、实际已空挡」。
      await ScreenshotService.updateShortcut(shortcut).catch(() => {});
      if (!aliveRef.current) return;
      setStatus({ kind: 'failed', message: getErrorMessage(err) });
    }
  }, [pending, shortcut]);

  const cancelPending = useCallback(() => {
    setPending(null);
    setRecording(false);
    setStatus({ kind: 'idle' });
  }, []);

  const commitRetention = useCallback(() => {
    const n = Number.parseInt(retentionDraft, 10);
    if (!Number.isFinite(n) || n < 0) {
      setRetentionDraft(String(retentionDays)); // 非法输入回填上次有效值，不落库
      return;
    }
    setRetentionDays(n);
    setRetentionDraft(String(n));
    void PreferencesService.set(SCREENSHOT_PREF_RETENTION_DAYS, n);
  }, [retentionDraft, retentionDays]);

  return {
    shortcut,
    pending,
    recording,
    retentionDraft,
    status,
    startRecording,
    stopRecording,
    onRecordKey,
    applyShortcut,
    cancelPending,
    setRetentionDraft,
    commitRetention,
  };
};

const getErrorMessage = (err: unknown): string => {
  if (err instanceof Error) return err.message;
  return String(err);
};
