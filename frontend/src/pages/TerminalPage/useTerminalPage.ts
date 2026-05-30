// TerminalPage 的 ViewModel：把「项目 + 选定 CLI」桥接到终端启动。
// 启动逻辑（新建 Tab + 记录历史）下沉到 TerminalContext.launchProject —— 跨页面启动的唯一事实源，
// Home 最近启动也走它。本 VM 仅做 Project → LaunchProjectInput 的形参适配（ProjectPanel 用 Project）。
import { useTerminal } from '@/terminal';
import { type Project } from '@/services';

export interface TerminalPageModel {
  /** 在活动面板启动一个项目终端（cliTool='none' = 裸 shell，其余 = 对应 CLI 适配器）。 */
  launchProject: (project: Project, cliTool: string) => void;
}

export function useTerminalPage(): TerminalPageModel {
  const { launchProject: ctxLaunch } = useTerminal();

  const launchProject = (project: Project, cliTool: string): void => {
    ctxLaunch({
      projectId: project.id,
      projectName: project.name,
      projectPath: project.path,
      alias: project.alias,
      cliTool,
    });
  };

  return { launchProject };
}
