// Composer 的 ViewModel：文本状态 + 按会话草稿持久化 + 提交状态机（借鉴 paseo composer/submit
// 的纯函数语义：乐观清空 → 失败回填）。不返回 JSX、不持翻译串（遵 MVVM 铁律）。
//
// 草稿（paseo draftKey 模式的最小落地）：
//   - 粒度 = 每会话一份（key `composer.draft:<sessionId>`），切会话各自恢复；
//   - 桌面走 preferences KV（SQLite；铁律禁 localStorage）；viewer 无 bindings 通道 →
//     模块级内存 Map 兜底（刷新即丢，与 paseo「草稿按端本地」的实际行为持平）;
//   - 异步水合用 generation 计数防竞态（快速切会话时旧草稿不污染新输入框，paseo 同款）；
//   - 空文本即删（不存垃圾行）。
import { useEffect, useRef, useState } from 'react';
import { PreferencesService } from '@/services';
import { isDesktop } from '@/services/transport';

const DRAFT_DEBOUNCE_MS = 600;

const draftKey = (sessionId: string): string => `composer.draft:${sessionId}`;

// viewer 内存草稿（无 bindings；铁律禁 localStorage —— 刷新即丢是当前合法下界）。
const memDrafts = new Map<string, string>();

async function loadDraft(sessionId: string): Promise<string> {
  if (!isDesktop) return memDrafts.get(sessionId) ?? '';
  return (await PreferencesService.get<string>(draftKey(sessionId))) ?? '';
}

async function saveDraft(sessionId: string, text: string): Promise<void> {
  if (!isDesktop) {
    if (text) memDrafts.set(sessionId, text);
    else memDrafts.delete(sessionId);
    return;
  }
  if (text) await PreferencesService.set(draftKey(sessionId), text);
  else await PreferencesService.delete(draftKey(sessionId));
}

export interface UseComposerArgs {
  sessionId: string;
  // 提交整句（父层做两段式 PTY 写入）。reject → 文本回填输入框（不丢稿）。
  onSubmit: (text: string) => Promise<void> | void;
}

export interface ComposerModel {
  text: string;
  setText: (v: string) => void;
  canSend: boolean;
  // 两段式写入进行中（亚秒级）：防双发交错（text1, text2, \r, \r 会串句）。
  sending: boolean;
  submit: () => void;
  onKeyDown: (e: React.KeyboardEvent) => void;
}

export function useComposer({ sessionId, onSubmit }: UseComposerArgs): ComposerModel {
  const [text, setText] = useState('');
  const [sending, setSending] = useState(false);
  const hydratedRef = useRef(false);
  const genRef = useRef(0);

  // 切会话：清空 → 异步恢复该会话草稿（generation 守门：水合回来时已切走则丢弃）。
  useEffect(() => {
    const gen = ++genRef.current;
    hydratedRef.current = false;
    setText('');
    if (!sessionId) return;
    void loadDraft(sessionId)
      .then((v) => {
        if (genRef.current !== gen) return;
        if (v) setText(v);
        hydratedRef.current = true;
      })
      .catch(() => {
        if (genRef.current === gen) hydratedRef.current = true;
      });
  }, [sessionId]);

  // 自动存稿（防抖）：仅水合完成后；空文本 = 删除草稿。
  useEffect(() => {
    if (!sessionId || !hydratedRef.current) return;
    const timer = setTimeout(() => {
      void saveDraft(sessionId, text.trim() ? text : '').catch(() => {});
    }, DRAFT_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [text, sessionId]);

  const submit = (): void => {
    const v = text.trim();
    if (!v || sending) return;
    // 乐观清空 + 立即清稿；失败把原文回填（paseo「失败回填比 toast 更不丢稿」）。
    setText('');
    void saveDraft(sessionId, '').catch(() => {});
    setSending(true);
    void Promise.resolve(onSubmit(v))
      .catch(() => setText(v))
      .finally(() => setSending(false));
  };

  const onKeyDown = (e: React.KeyboardEvent): void => {
    // Enter 发送，Shift+Enter 换行；IME 合成中（中文/日文）必须先判，不抢回车。
    if (e.key === 'Enter' && !e.shiftKey && !(e.nativeEvent as { isComposing?: boolean }).isComposing) {
      e.preventDefault();
      submit();
    }
  };

  return { text, setText, canSend: !!text.trim() && !sending, sending, submit, onKeyDown };
}
