// SessionSubList 的 ViewModel —— 移植 cc-pane SessionSubList 的数据/动作（local 扫盘子集）。
// 按需扫盘（首次挂载拉一次）+ 恢复会话（launchProject 带 resumeId → CLI resume）+ 复制 id。
// 不返回 JSX、不持翻译串（遵 MVVM）。
import { useEffect, useState } from 'react';
import { SessionScanService, type ResumeSession } from '@/services';
import { useRouter } from '@/router';
import { useTerminal } from '@/terminal';

// 启动目标：项目节点自身 id 作 launchProject.projectId（软关联，与右键启动一致）。
export interface SubListTarget {
  id: string;
  name: string;
  path: string;
}

export interface SessionSubListModel {
  sessions: ResumeSession[];
  loading: boolean;
  /** 恢复一条会话；override 用于「以 reclaude 恢复」（仅 claude 会话）。 */
  resume: (session: ResumeSession, cliToolOverride?: string) => void;
  copyId: (session: ResumeSession) => void;
}

// resume id 合法性：非空且不含 ':'（裸 id 才能透传 CLI；对照 cc-pane buildResumeOptions 守门）。
function isValidResumeId(id: string): boolean {
  return id !== '' && !id.includes(':');
}

export function useSessionSubList(target: SubListTarget): SessionSubListModel {
  const { launchProject } = useTerminal();
  const { navigate } = useRouter();
  const [sessions, setSessions] = useState<ResumeSession[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    SessionScanService.listProjectSessions(target.path)
      .then((list) => {
        if (alive) {
          setSessions(list);
          setLoading(false);
        }
      })
      .catch(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [target.path]);

  const resume = (session: ResumeSession, cliToolOverride?: string): void => {
    if (!isValidResumeId(session.id)) return;
    launchProject({
      projectId: target.id,
      projectName: target.name,
      projectPath: target.path,
      cliTool: cliToolOverride ?? session.source,
      resumeId: session.id,
    });
    navigate('terminal');
  };

  const copyId = (session: ResumeSession): void => {
    void navigator.clipboard?.writeText(session.id).catch(() => {});
  };

  return { sessions, loading, resume, copyId };
}
