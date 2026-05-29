import type { Provider, ProviderInput } from '@/services';

// ProviderContextValue：接入 Provider 的全局状态 + 动作（弃 Zustand，纯 Context；
// 范式同 ProjectProvider）。
//
// 动作约定：调后端 → 刷新列表（与后端插入序 + default 不变式保持一致，不乐观拼接）。
// 错误**向上抛**，由 View 用 NativeDialogs 提示（如 SetDefault 未知 id / 删除报错），
// Provider 不吞错。明文 apiKey 仅在 input 里上行；列表是脱敏 Provider（hasApiKey）。
export interface ProviderContextValue {
  providers: Provider[];
  loading: boolean; // 仅首屏拉取期 true；mutation 刷新不切
  refresh: () => Promise<void>;
  addProvider: (input: ProviderInput) => Promise<Provider>;
  updateProvider: (id: string, input: ProviderInput) => Promise<void>;
  removeProvider: (id: string) => Promise<void>;
  setDefaultProvider: (id: string) => Promise<void>;
}
