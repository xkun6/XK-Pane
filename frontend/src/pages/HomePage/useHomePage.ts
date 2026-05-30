import { useEffect, useState } from 'react';
import { LaunchHistoryService, type CliTool, type LaunchRecord } from '@/services';
import { useProjects } from '@/project';
import { useCliTools } from '@/shared/hooks/useCliTools';
import { useRouter } from '@/router';

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
}

export const useHomePage = (): UseHomePageResult => {
  const { projects } = useProjects();
  const { tools, loading: toolsLoading } = useCliTools();
  const { navigate } = useRouter();
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

  return {
    greetingKey,
    projectCount: projects.length,
    recentLaunches,
    launchesLoading,
    tools,
    toolsLoading,
    navigate,
  };
};
