import { Service as Binding } from '@bindings/xkpane/internal/services/botbridge';

export interface BotBridgeCredentialField {
  key: string;
  secret: boolean;
}

export interface BotBridgePlatformSpec {
  id: string;
  fields: BotBridgeCredentialField[];
}

export interface BotBridgeStatus {
  enabled: boolean;
  platform: string;
  /** 仅非敏感凭据字段；敏感字段（密钥）后端不回传。 */
  credentials: Record<string, string>;
  connected: boolean;
  error?: string;
}

export interface BotBridgeConfigInput {
  platform: string;
  credentials: Record<string, string>;
  enabled: boolean;
}

export const BotBridgeService = {
  async listPlatforms(): Promise<BotBridgePlatformSpec[]> {
    const specs = await Binding.ListPlatforms();
    return (specs ?? []).map((sp) => ({
      id: sp.id ?? '',
      fields: (sp.fields ?? []).map((f) => ({ key: f.key ?? '', secret: !!f.secret })),
    }));
  },

  async getStatus(): Promise<BotBridgeStatus> {
    const s = await Binding.GetStatus();
    return {
      enabled: !!s.enabled,
      platform: s.platform ?? 'feishu',
      credentials: (s.credentials ?? {}) as Record<string, string>,
      connected: !!s.connected,
      error: s.error ?? '',
    };
  },

  async setConfig(input: BotBridgeConfigInput): Promise<void> {
    await Binding.SetConfig(input);
  },
} as const;
