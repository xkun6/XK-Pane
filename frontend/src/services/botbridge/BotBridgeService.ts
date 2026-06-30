import { Service as Binding } from '@bindings/xkpane/internal/services/botbridge';

export interface BotBridgeStatus {
  enabled: boolean;
  platform: string;
  appId: string;
  connected: boolean;
  error?: string;
}

export interface BotBridgeConfigInput {
  platform: string;
  appId: string;
  appSecret: string;
  enabled: boolean;
}

export const BotBridgeService = {
  async getStatus(): Promise<BotBridgeStatus> {
    const s = await Binding.GetStatus();
    return {
      enabled: !!s.enabled,
      platform: s.platform ?? 'feishu',
      appId: s.appId ?? '',
      connected: !!s.connected,
      error: s.error ?? '',
    };
  },

  async setConfig(input: BotBridgeConfigInput): Promise<void> {
    await Binding.SetConfig(input);
  },
} as const;

