// OrchestratorService：编排事件 + Web 发起启动的前端入口。
//
// 后端 launch_task（MCP）或 POST /api/launch（Web）在 PTY 里建好会话后广播
// `orchestrator:launch`。所有前端（桌面 + Web）据此开一个新 Tab 并**附着到这个已存在的
// sessionId**（不再 CreateSession），让被启动的会话在两端同步可见（P7 会话级互通）。
//
// transport（P7 M2）：onLaunch 经 eventTransport（桌面 Wails 总线 / 浏览器 /ws 镜像）。
// launch 仅浏览器用（viewer 经 POST /api/launch；桌面用本地 CreateSession 路径，无此 binding）。
import { eventTransport, httpClient, isDesktop } from '@/services/transport';
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

// LaunchInput 对齐后端 POST /api/launch 请求体（orchestrator.LaunchParams 的 json 形态）。
export interface LaunchInput {
  projectPath: string;
  prompt?: string;
  resumeId?: string;
  cliTool?: string;
  workspaceName?: string;
  providerId?: string;
  title?: string;
}

export interface LaunchResult {
  taskId: string;
  sessionId: string;
}

export const OrchestratorService = {
  // 订阅 orchestrator:launch；每条事件回调一次。返回取消函数（务必在 useEffect 里 return）。
  onLaunch(handler: (payload: OrchestratorLaunchPayload) => void): () => void {
    return eventTransport.on<OrchestratorLaunchPayload>(AppEvents.OrchestratorLaunch, (data) => {
      if (data && data.sessionId) handler(data);
    });
  },

  // Web（viewer）发起启动：POST /api/launch → 后端 LaunchTask → 广播 orchestrator:launch →
  // 桌面与本 Web 端同时开窗附着同 sid。桌面端用本地 CreateSession 路径，不调用此方法。
  async launch(input: LaunchInput): Promise<LaunchResult> {
    if (isDesktop) {
      return Promise.reject(new Error('桌面端请走本地启动（launchProject），不经 /api/launch'));
    }
    return httpClient.post<LaunchResult>('/api/launch', {
      projectPath: input.projectPath,
      prompt: input.prompt ?? '',
      resumeId: input.resumeId ?? '',
      cliTool: input.cliTool ?? '',
      workspaceName: input.workspaceName ?? '',
      providerId: input.providerId ?? '',
      title: input.title ?? '',
    });
  },
} as const;
