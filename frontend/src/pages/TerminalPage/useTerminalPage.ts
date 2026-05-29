// TerminalPage 的 ViewModel：把「点击项目」桥接到终端——在活动面板新建一个终端 Tab。
// 经 useTerminal（Context）；不直接 import bindings。projectPath = PTY cwd（铁律 D7），
// reconcile effect 据此起会话。MVP 用裸 shell（cliTool 'none'）；选 CLI 工具留 3b。
import { useTerminal } from '@/terminal';
import { LaunchHistoryService, type Project } from '@/services';

export interface TerminalPageModel {
  launchProject: (project: Project) => void;
}

export function useTerminalPage(): TerminalPageModel {
  const { activePaneId, newTab } = useTerminal();

  const launchProject = (project: Project): void => {
    newTab(activePaneId, {
      projectId: project.id,
      projectPath: project.path,
      cliTool: 'none',
      title: project.alias ?? project.name,
    });
    // 记录启动历史（fire-and-forget：记录失败绝不阻断终端启动）。
    void LaunchHistoryService.record({
      projectId: project.id,
      projectName: project.name,
      projectPath: project.path,
      cliTool: 'none',
      runtimeKind: 'local',
    }).catch(() => {});
  };

  return { launchProject };
}
