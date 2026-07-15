import type { Messages } from '@/i18n';

export const providerSettingsZhCN: Messages = {
  settings: {
    provider: {
      tabs: {
        claude: 'Claude Code',
        codex: 'Codex',
        other: '其它 CLI',
      },
      // cc-switch 式全局配置 Provider（Claude Code / Codex 两标签共用）。
      global: {
        add: '新增',
        importCurrent: '从当前配置导入',
        hint: '切换会直接写入该 CLI 的全局配置文件，影响之后启动的所有该 CLI 会话（包括 XK-Pane 之外）。',
        empty: '暂无 Provider，可点击「从当前配置导入」快速开始。',
        currentBadge: '当前',
        category: {
          official: '官方',
          custom: '自定义',
          imported: '已导入',
        },
        actions: {
          switch: '切换',
          edit: '编辑',
          remove: '删除',
        },
        removeConfirm: {
          title: '删除 Provider',
          message: '确定删除 Provider「{{name}}」吗？该操作不可撤销。',
          ok: '删除',
          cancel: '取消',
        },
        form: {
          addTitle: '新增 {{app}} Provider',
          editTitle: '编辑 {{app}} Provider',
          name: '名称',
          namePlaceholder: '例如：官方直连',
          notes: '备注（可选）',
          notesPlaceholder: '给自己看的备注',
          currentEditWarning: '这是当前正在使用的 {{app}} Provider，保存后将立即同步写入全局配置文件。',
          template: '模板',
          templates: {
            blank: '空白',
            apiKey: 'API Key 直连',
            thirdParty: '第三方中转',
          },
          templateOverwriteConfirm: {
            title: '覆盖当前编辑内容',
            message: '应用模板会覆盖下方编辑器中已填写的内容，确定继续吗？',
            ok: '覆盖',
            cancel: '取消',
          },
          settingsJson: 'settings.json 配置',
          jsonEmptyError: '不能为空，至少填 {}',
          jsonInvalidError: '不是合法的 JSON 对象',
          authJson: 'auth.json（登录态，可选）',
          authJsonHelper: '留空 = 不管理登录态（切换到该 Provider 时不会改动 auth.json）',
          configToml: 'config.toml',
          save: '保存',
          cancel: '取消',
        },
      },
      // 「其它 CLI」标签页（按会话注入类型：gemini / kimi / glm / cursor / opencode）。
      hint: '管理用于启动 CLI 的接入凭据（API Key / 端点）。密钥经 AES-GCM 加密存于本地数据库，绝不明文回显——编辑时密钥框留空即保留原值。',
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
        namePlaceholder: '例如：我的 Gemini',
        type: '类型',
        apiKey: 'API Key',
        apiKeyPlaceholderNew: '粘贴你的 API Key',
        apiKeyPlaceholderEdit: '留空 = 保留原密钥',
        baseUrl: '端点 URL（可选）',
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
