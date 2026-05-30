// TerminalPage 的 ViewModel：把「项目 + 选定 CLI」桥接到终端——在活动面板新建一个终端 Tab。
// 经 useTerminal（Context）；不直接 import bindings。projectPath = PTY cwd（铁律 D7），
// reconcile effect 据 tab 的 projectPath + cliTool 起会话。cliTool 由启动菜单选定（'none' = 裸 shell）。
import { useTerminal } from '@/terminal';
import { LaunchHistoryService, type Project } from '@/services';

export interface TerminalPageModel {
  /** 在活动面板启动一个项目终端（cliTool='none' = 裸 shell，其余 = 对应 CLI 适配器）。 */
  launchProject: (project: Project, cliTool: string) => void;
}

export function useTerminalPage(): TerminalPageModel {
  const { activePaneId, newTab } = useTerminal();

  const launchProject = (project: Project, cliTool: string): void => {
    const base = project.alias ?? project.name;
    const title = cliTool && cliTool !== 'none' ? `${base} (${cliTool})` : base;
    newTab(activePaneId, {
      projectId: project.id,
      projectPath: project.path,
      cliTool,
      title,
    });
    // 记录启动历史（fire-and-forget：记录失败绝不阻断终端启动）。
    void LaunchHistoryService.record({
      projectId: project.id,
      projectName: project.name,
      projectPath: project.path,
      cliTool,
      runtimeKind: 'local',
    }).catch(() => {});
  };

  return { launchProject };
}
