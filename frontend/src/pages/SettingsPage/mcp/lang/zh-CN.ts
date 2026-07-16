import type { Messages } from '@/i18n';

export const mcpZhCN: Messages = {
  settings: {
    mcp: {
      title: 'MCP 管理',
      hint: 'MCP server 以 XK-Pane 为唯一事实源（SSOT），按下方开关分别投影到 Claude Code（~/.claude.json 的 mcpServers）与 Codex（~/.codex/config.toml 的 [mcp_servers]）。关闭开关 = 从对应 CLI 移除该 server，但 XK-Pane 仍保留配置。',
      add: '新增',
      importCurrent: '从现有配置导入',
      empty: '暂无 MCP server，可点击「从现有配置导入」快速开始，或手动新增。',
      noSummary: '—',
      toggles: {
        claude: 'Claude',
        codex: 'Codex',
      },
      actions: {
        edit: '编辑',
        remove: '删除',
      },
      removeConfirm: {
        title: '删除 MCP server',
        message: '确定删除 MCP server「{{name}}」吗？将从已启用的 CLI 中移除，该操作不可撤销。',
        ok: '删除',
        cancel: '取消',
      },
      importDone: {
        title: '导入完成',
        message: '新增了 {{count}} 个 MCP server。',
      },
      form: {
        addTitle: '新增 MCP server',
        editTitle: '编辑 MCP server',
        name: '名称',
        namePlaceholder: '例如：filesystem',
        nameError: '仅允许字母、数字、点、下划线、连字符，且非空',
        serverJson: 'Server 定义 JSON',
        serverJsonPlaceholder:
          '{"type": "sse", "url": "http://..."}，支持直接粘贴 {"servers": {...}} / {"mcpServers": {...}} 片段',
        jsonEmptyError: '不能为空',
        jsonInvalidError: '不是合法的 JSON 对象',
        jsonMultipleError: '检测到多个 server 定义，请一次只添加一个',
        enableClaude: '启用于 Claude Code',
        enableCodex: '启用于 Codex',
        save: '保存',
        cancel: '取消',
      },
      error: {
        title: 'MCP server 操作失败',
      },
    },
  },
};
