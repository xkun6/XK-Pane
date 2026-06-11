// useCliTools：拉取并缓存 CLI 工具检测结果（对齐 cc-pane web/hooks/useCliTools.ts）。
//
// DetectAll 会跑 which + --version 探测（每工具最多 5s），故模块级缓存一次：首个消费者触发
// 拉取，后续消费者复用缓存，避免每次打开启动菜单都重新探测。
import { useEffect, useState } from 'react';
import { CliToolService, type CliTool } from '@/services';

let cache: CliTool[] | null = null;
let inflight: Promise<CliTool[]> | null = null;

function load(): Promise<CliTool[]> {
  if (cache) return Promise.resolve(cache);
  if (!inflight) {
    inflight = CliToolService.detectAll()
      .then((tools) => {
        cache = tools;
        inflight = null;
        return tools;
      })
      .catch((err) => {
        inflight = null;
        throw err;
      });
  }
  return inflight;
}

interface UseCliToolsResult {
  tools: CliTool[];
  loading: boolean;
}

export function useCliTools(): UseCliToolsResult {
  const [tools, setTools] = useState<CliTool[]>(cache ?? []);
  const [loading, setLoading] = useState<boolean>(cache === null);

  useEffect(() => {
    if (cache) {
      setTools(cache);
      setLoading(false);
      return;
    }
    let alive = true;
    load()
      .then((t) => {
        if (alive) {
          setTools(t);
          setLoading(false);
        }
      })
      .catch(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  return { tools, loading };
}
