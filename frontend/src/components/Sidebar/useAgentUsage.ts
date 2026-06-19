import { useEffect, useState } from 'react';
import { AgentUsageService, type TokenUsage } from '@/services';
import { isDesktop } from '@/services/transport';

// 侧栏 token 用量轮询：挂载即拉一次，之后每 60s 刷新（后端自带 30s 缓存，轮询很廉价）。
// 非桌面端不轮询（viewer 无本机文件系统，service 也会返回 null）。
const REFRESH_MS = 60_000;

export const useAgentUsage = (): { usage: TokenUsage | null } => {
  const [usage, setUsage] = useState<TokenUsage | null>(null);

  useEffect(() => {
    if (!isDesktop) return;
    let alive = true;
    const tick = async () => {
      try {
        const next = await AgentUsageService.getTokenUsage();
        if (alive) setUsage(next);
      } catch {
        // best-effort：取数失败保持上次值，不打断侧栏渲染
      }
    };
    void tick();
    const id = window.setInterval(() => void tick(), REFRESH_MS);
    return () => {
      alive = false;
      window.clearInterval(id);
    };
  }, []);

  return { usage };
};
