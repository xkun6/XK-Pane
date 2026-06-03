import { useCallback, useEffect, useState } from 'react';
import { LaunchHistoryService, type CliTool, type LaunchRecord } from '@/services';
import { useProjects } from '@/project';
import { useCliTools } from '@/shared/hooks/useCliTools';
import { useRouter } from '@/router';
import { useRightPanel, type RightPanelId } from '@/rightpanel';
import { useTerminal } from '@/terminal';

// HomePage ViewModel：仪表盘数据装配 —— 时段问候 + 项目数 + 最近启动 + CLI 环境检测 + 导航。
// 复用既有数据源（LaunchHistoryService / ProjectProvider / useCliTools / Router），不新建后端。
// 不返回 JSX、不持翻译串（问候按 key 暴露，由 View 调 t()）。

export type GreetingKey = 'morning' | 'afternoon' | 'evening';

const RECENT_LIMIT = 6;

function greetingForHour(hour: number): GreetingKey {
  if (hour < 12) return 'morning';
  if (hour < 18) return 'afternoon';
  return 'evening';
}

export interface UseHomePageResult {
  greetingKey: GreetingKey;
  projectCount: number;
  recentLaunches: LaunchRecord[];
  launchesLoading: boolean;
  tools: CliTool[];
  toolsLoading: boolean;
  navigate: (id: string) => void;
  /** 打开右侧抽屉（Git / 本地历史）。git/history 已是面板而非页面，不能 navigate。 */
  openPanel: (id: RightPanelId) => void;
  /** 启动一条最近记录：有回填的 resumeSessionId 则 resume 续会话，否则新建；并跳到终端页。 */
  relaunch: (r: LaunchRecord) => void;
}

export const useHomePage = (): UseHomePageResult => {
  const { projects } = useProjects();
  const { tools, loading: toolsLoading } = useCliTools();
  const { navigate } = useRouter();
  const { open: openPanel } = useRightPanel();
  const { launchProject } = useTerminal();
  const [recentLaunches, setRecentLaunches] = useState<LaunchRecord[]>([]);
  const [launchesLoading, setLaunchesLoading] = useState(true);
  // 问候按首次挂载的本地小时定（home 非保活，每次进入重算；见 routes.tsx）。
  const [greetingKey] = useState<GreetingKey>(() => greetingForHour(new Date().getHours()));

  useEffect(() => {
    let alive = true;
    LaunchHistoryService.list(RECENT_LIMIT)
      .then((rows) => {
        if (alive) {
          setRecentLaunches(rows);
          setLaunchesLoading(false);
        }
      })
      .catch(() => {
        if (alive) setLaunchesLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  const relaunch = useCallback(
    (r: LaunchRecord): void => {
      // 回填过 resumeSessionId 则续会话（裸 id 才能透传 CLI resume，含 ':' 的丢弃，对照 SessionSubList）；
      // 未回填（空）则 resumeId=undefined → 走新会话，与旧行为一致。
      const resumeId =
        r.resumeSessionId && !r.resumeSessionId.includes(':') ? r.resumeSessionId : undefined;
      launchProject({
        projectId: r.projectId,
        projectName: r.projectName,
        projectPath: r.projectPath,
        cliTool: r.cliTool,
        resumeId,
      });
      navigate('terminal');
    },
    [launchProject, navigate],
  );

  return {
    greetingKey,
    projectCount: projects.length,
    recentLaunches,
    launchesLoading,
    tools,
    toolsLoading,
    navigate,
    openPanel,
    relaunch,
  };
};
