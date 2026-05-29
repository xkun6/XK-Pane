// TerminalPage 的 ViewModel：MVP 启动入口——选文件夹后在活动面板新建一个裸 shell 终端 Tab。
// 经 services 层（NativeDialogs）与 useTerminal（Context）；不直接 import bindings。
// dialogTitle 由 View 传入（VM 不持有翻译串，遵循 MVVM 铁律）。
import { NativeDialogs } from '@/services';
import { useTerminal } from '@/terminal';

export interface TerminalPageModel {
  openFolder: (dialogTitle: string) => Promise<void>;
}

export function useTerminalPage(): TerminalPageModel {
  const { activePaneId, newTab } = useTerminal();

  const openFolder = async (dialogTitle: string): Promise<void> => {
    const dir = await NativeDialogs.openDirectory({ title: dialogTitle });
    if (!dir) return;
    // 裸 shell（cliTool 'none'）：projectPath = PTY cwd（铁律 D7）。reconcile effect 据此起会话。
    newTab(activePaneId, { projectId: dir, projectPath: dir, cliTool: 'none' });
  };

  return { openFolder };
}
