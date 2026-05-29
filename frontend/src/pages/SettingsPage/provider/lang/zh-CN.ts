import type { Messages } from '@/i18n';

export const providerSettingsZhCN: Messages = {
  settings: {
    provider: {
      title: '接入 Provider',
      hint: '管理用于启动 CLI 的接入凭据（API Key / 端点 / 云配置）。密钥经 AES-GCM 加密存于本地数据库，绝不明文回显——编辑时密钥框留空即保留原值。',
      add: '添加 Provider',
      empty: '暂无 Provider。点「添加 Provider」配置一个。',
      defaultBadge: '默认',
      keySet: '已设密钥',
      actions: {
        setDefault: '设为默认',
        edit: '编辑',
        remove: '删除',
      },
      types: {
        anthropic: 'Anthropic',
        bedrock: 'AWS Bedrock',
        vertex: 'Google Vertex',
        proxy: '代理（Anthropic 兼容）',
        config_profile: '配置目录（ccswitch）',
        open_ai: 'OpenAI / Codex',
        gemini: 'Gemini',
        kimi: 'Kimi',
        glm: 'GLM（智谱）',
        cursor: 'Cursor',
        opencode: 'OpenCode',
      },
      form: {
        addTitle: '添加 Provider',
        editTitle: '编辑 Provider',
        name: '名称',
        namePlaceholder: '例如：我的 Anthropic',
        type: '类型',
        apiKey: 'API Key',
        apiKeyPlaceholderNew: '粘贴你的 API Key',
        apiKeyPlaceholderEdit: '留空 = 保留原密钥',
        baseUrl: '端点 URL（可选）',
        region: '区域（Region）',
        awsProfile: 'AWS Profile',
        projectId: 'GCP 项目 ID',
        configDir: '配置目录路径',
        isDefault: '设为默认 Provider',
        save: '保存',
        cancel: '取消',
      },
      removeConfirm: {
        title: '删除 Provider',
        message: '确定删除 Provider「{{name}}」吗？该操作不可撤销。',
        ok: '删除',
        cancel: '取消',
      },
      error: {
        title: 'Provider 操作失败',
      },
    },
  },
};
