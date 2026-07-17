import { useState } from 'react';

export type ProviderTabId = 'claude' | 'codex' | 'grok' | 'other';

export interface UseProviderSettingsResult {
  tab: ProviderTabId;
  setTab: (tab: ProviderTabId) => void;
}

// 顶层 Provider 设置 ViewModel：仅管四标签页（Claude Code / Codex / Grok Build / 其它 CLI）
// 的激活态。
// 各标签页自己的业务状态下沉到对应组件（GlobalProviderPanel / OtherCliProviders），
// 本 hook 不持有它们的数据。
export const useProviderSettings = (): UseProviderSettingsResult => {
  const [tab, setTab] = useState<ProviderTabId>('claude');
  return { tab, setTab };
};
