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
// 路径写进当前活跃终端。CLI 会话（claude/reclaude/codex）须包 bracketed-paste 序列——
// 它们只在「真粘贴事件」里把图片路径转成 [Image #N] 附件，裸写会留成路径字符串（实测）；
// 非 CLI 会话写裸路径。无活跃会话 / SSH 会话（远端读不到本机文件）时仅提示「已复制到
// 剪贴板」（后端已把图片文件 CF_HDROP 写入剪贴板）。桌面专属：onSaved 在 viewer 端 no-op。
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
      // CLI 会话包 bracketed-paste（等价 cc-pane 的 term.paste()），触发其粘贴管线把
      // 图片路径转 [Image #N] 附件；裸 shell 不包（避免 ESC 序列当垃圾字符进 readline）。
      const isCliSession = !!tab?.cliTool && tab.cliTool !== 'none';
      const text = isCliSession ? `\x1b[200~${filePath}\x1b[201~` : filePath;
      TerminalService.write(sessionId, text)
        .then(() => setNotice('pasted'))
        .catch(() => setNotice('copied')); // 写入失败时剪贴板仍有图片文件可手动粘贴
    });
  }, []);

  const clearNotice = useCallback(() => setNotice(null), []);
  return { notice, clearNotice };
};
