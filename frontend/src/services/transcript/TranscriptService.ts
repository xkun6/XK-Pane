// TranscriptService：拉取活跃会话的「结构化对话时间线」（Web/手机的重排视图数据源）。
//
// 为什么存在：XK-Pane 的 PTY 镜像是按某宽度排好版的原始字节（绝对光标定位 + alt-screen），
// 与宽度焊死、无法在窄屏重排。而后端 GET /api/sessions/timeline 从 Claude 的 JSONL 转录
// （排版之前的结构化真相）解析出 user/assistant/thinking/tool 事件，可在手机上自由重排。
// 详见记忆 xkpane-mobile-strategy / 后端 internal/services/transcript。
//
// 仅 Web（viewer）使用：桌面镜像的是 PTY 网格，直接返回 unsupported（前端会回退 xterm）。
// after = 已消费行数游标，append-only 的 jsonl 保证增量轮询不漏不重。
import { httpClient, isDesktop } from '@/services/transport';

export type TimelineKind = 'user' | 'assistant' | 'thinking' | 'tool_use' | 'tool_result';

export interface TimelineEvent {
  seq: number;
  uuid?: string;
  kind: TimelineKind;
  text?: string;
  toolName?: string;
  toolInput?: string;
  toolUseId?: string;
  timestamp?: string;
}

export interface Timeline {
  supported: boolean;
  events: TimelineEvent[];
  total: number; // 文件总行数 = 下次轮询的 after 游标
  title?: string;
}

interface TimelineDTO {
  supported?: boolean;
  events?: TimelineEvent[];
  total?: number;
  title?: string;
}

export const TranscriptService = {
  // 拉取 [after, 末尾) 的结构化时间线事件。桌面端无此视图 → 直接 unsupported。
  async getTimeline(sessionId: string, after = 0): Promise<Timeline> {
    if (isDesktop) {
      return { supported: false, events: [], total: 0 };
    }
    const res = await httpClient.get<TimelineDTO>(
      `/api/sessions/timeline?sessionId=${encodeURIComponent(sessionId)}&after=${after}`,
    );
    return {
      supported: res.supported === true,
      events: res.events ?? [],
      total: Number(res.total ?? 0),
      title: res.title,
    };
  },
} as const;
