// AgentUsageService：取本机 Claude Code 会话日志统计出的 token 用量（近5h / 今日 / 本周）。
//
// 自包含：后端 internal/services/agentusage 直接增量解析 ~/.claude/projects/**/*.jsonl 的
// assistant usage，不依赖 claude-hud 插件、也不依赖任何 CLI 在跑。口径 = fanbox-win 的
// 「token 总量 · 本地会话日志统计」。整机所有 Claude Code 会话累计（不限 XK-Pane 自己）。
//
// 仅桌面（owner）可用：viewer（手机/Web）无本机文件系统访问，getTokenUsage 直接返回 null，
// 侧栏 widget 据此不渲染。桌面走 Wails binding agentusage.GetTokenUsage。
import { isDesktop } from '@/services/transport';
import { Service as AgentUsageBinding } from '@bindings/xkpane/internal/services/agentusage';

// 单个时间窗口的 token 聚合（对齐后端 agentusage.TokenWindow）。Total = 四类 token 之和。
export interface TokenWindow {
  total: number;
  input: number;
  output: number;
  cacheRead: number;
  cacheCreate: number;
  msgs: number;
}

// 四窗口聚合（对齐后端 agentusage.TokenUsage）。available=false 表示没找到 ~/.claude/projects。
export interface TokenUsage {
  available: boolean;
  at: number;
  last5h: TokenWindow;
  today: TokenWindow;
  week: TokenWindow;
  month: TokenWindow;
}

export const AgentUsageService = {
  // 取一次 token 用量快照。非桌面端直接 null；后端自带 30s 缓存，前端可放心按 60s 轮询。
  async getTokenUsage(): Promise<TokenUsage | null> {
    if (!isDesktop) return null;
    const res = await AgentUsageBinding.GetTokenUsage();
    return (res as TokenUsage | null) ?? null;
  },
} as const;
