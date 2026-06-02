import { useEffect } from 'react';
import { isMac } from '@/shared/platform';
import { lifecycleRole } from '@/services/transport';
import { useRouter } from '@/router';
import { useRightPanel } from '@/rightpanel';

// 全局快捷键（挂在 AppLayout，常驻整个会话）：
//  - Mod+1..9 ：切换 Sidebar 主导航页（按可见顺序，跳过面板型条目）
//  - Mod+,    ：打开设置
//  - Mod+Shift+G / Mod+Shift+H：开 / 关 Git / 本地历史右侧抽屉
//  - Esc      ：收起当前右侧抽屉
//
// Mod = Windows/Linux 的 Ctrl、macOS 的 ⌘。终端（xterm）聚焦时让行（终端独占按键）；
// Monaco 仅占用 Mod+S，与此处不冲突。展示用组合键见 shared/keyboardShortcuts.ts。

function inTerminal(el: Element | null): boolean {
  return !!el?.closest('.xterm');
}

function inEditable(el: Element | null): boolean {
  if (!el) return false;
  if (el.closest('.xterm') || el.closest('.monaco-editor')) return true;
  const tag = el.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || (el as HTMLElement).isContentEditable;
}

export function useGlobalShortcuts(): void {
  const { primary, navigate } = useRouter();
  const { toggle, active, close } = useRightPanel();

  useEffect(() => {
    // 可数字导航的页：主导航中的非面板条目（与 Sidebar 渲染顺序一致；Web 隐藏 desktopOnly）。
    const navItems = primary.filter(
      (r) => r.panel === undefined && !(r.desktopOnly && lifecycleRole === 'viewer'),
    );

    const onKeyDown = (e: KeyboardEvent): void => {
      const target = e.target as Element | null;

      // Esc：收起抽屉（仅当有抽屉打开且焦点不在可编辑区 / 终端 —— 不抢输入框 / Monaco 的 Esc）
      if (e.key === 'Escape') {
        if (active && !inEditable(target)) {
          close();
          e.preventDefault();
        }
        return;
      }

      const mod = isMac ? e.metaKey : e.ctrlKey;
      const wrongMod = isMac ? e.ctrlKey : e.metaKey;
      if (!mod || wrongMod || e.altKey) return;
      if (inTerminal(target)) return; // 终端独占其按键组合

      if (e.shiftKey) {
        const k = e.key.toLowerCase();
        if (k === 'g') {
          toggle('git');
          e.preventDefault();
        } else if (k === 'h') {
          toggle('history');
          e.preventDefault();
        }
        return;
      }

      if (e.key === ',') {
        navigate('settings');
        e.preventDefault();
        return;
      }

      if (/^[1-9]$/.test(e.key)) {
        const route = navItems[Number(e.key) - 1];
        if (route) {
          navigate(route.id);
          e.preventDefault();
        }
      }
    };

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [primary, navigate, toggle, active, close]);
}
