import { useCallback, useEffect, useRef, useState } from 'react';
import { ScreenshotService, TerminalService } from '@/services';
import { useTerminal } from '@/terminal';
import { findPane } from '@/terminal/paneTree';

/** snackbar 提示类别（View 用 t(`screenshotBridge.notice.${notice}`) 渲染）。 */
export type ScreenshotNotice = 'pasted' | 'copied';

export interface UseScreenshotBridgeResult {
  notice: ScreenshotNotice | null;
  clearNotice: () => void;
}

// 截图快捷键闭环桥（比 cc-pane 多走的一步）：订阅后端 screenshot:saved，把已落盘 PNG 的
// 路径直接写进当前活跃终端（claude/codex 整段识别路径 token，容忍内部空格）；无活跃会话 /
// SSH 会话（远端读不到本机文件）时仅提示「已复制到剪贴板」（后端已兜底写剪贴板）。
// 桌面专属：ScreenshotService.onSaved 在 viewer 端是 no-op。
export const useScreenshotBridge = (): UseScreenshotBridgeResult => {
  const { rootPane, activePaneId } = useTerminal();
  const [notice, setNotice] = useState<ScreenshotNotice | null>(null);

  // 面板树用 ref 持有，避免树每次变化都重订阅事件。
  const treeRef = useRef({ rootPane, activePaneId });
  treeRef.current = { rootPane, activePaneId };

  useEffect(() => {
    return ScreenshotService.onSaved((payload) => {
      const filePath = payload?.filePath;
      if (!filePath) return;

      const tree = treeRef.current;
      const pane = findPane(tree.rootPane, tree.activePaneId);
      const tab =
        pane?.type === 'panel' ? pane.tabs.find((t) => t.id === pane.activeTabId) : undefined;
      // SSH 会话跑在远端，本机截图路径对它无意义 → 退回剪贴板提示。
      const sessionId = tab && tab.runtimeKind !== 'ssh' ? (tab.sessionId ?? null) : null;

      if (!sessionId) {
        setNotice('copied');
        return;
      }
      // 裸路径写入 PTY（与 cc-pane 粘贴流同格式，不加引号——claude/codex 均整段识别）。
      TerminalService.write(sessionId, filePath)
        .then(() => setNotice('pasted'))
        .catch(() => setNotice('copied')); // 写入失败时剪贴板仍有路径可手动粘贴
    });
  }, []);

  const clearNotice = useCallback(() => setNotice(null), []);
  return { notice, clearNotice };
};
