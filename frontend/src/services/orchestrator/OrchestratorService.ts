// OrchestratorService：编排事件的前端入口。
//
// 后端 MCP launch_task 在 PTY 里创建好会话后广播 `orchestrator:launch`
// （events.go RegisterEvent[OrchestratorLaunchPayload]）。前端据此开一个新 Tab 并
// **附着到这个已存在的 sessionId**（不再调 CreateSession），让被编排启动的子会话在界面可见。
//
// 与 terminal 事件不同：这是「单事件、无 sessionId 过滤」的广播，订阅后逐条处理。
import { Events } from '@wailsio/runtime';

import { AppEvents } from '@/shared/events';

// 对齐后端 events.OrchestratorLaunchPayload。
export interface OrchestratorLaunchPayload {
  taskId: string;
  sessionId: string;
  projectPath: string;
  workspaceName?: string;
  providerId?: string;
  title?: string;
  resumeId?: string;
  cliTool?: string;
}

interface WailsEventPayload<T> {
  data: T;
  name: string;
  sender?: string;
}

export const OrchestratorService = {
  // 订阅 orchestrator:launch；每条事件回调一次。返回取消函数（务必在 useEffect 里 return）。
  onLaunch(handler: (payload: OrchestratorLaunchPayload) => void): () => void {
    const off = Events.On(
      AppEvents.OrchestratorLaunch,
      (e: WailsEventPayload<OrchestratorLaunchPayload>) => {
        const data = e?.data;
        if (data && data.sessionId) handler(data);
      },
    );
    return () => {
      if (typeof off === 'function') off();
    };
  },
} as const;
