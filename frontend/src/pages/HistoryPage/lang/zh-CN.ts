import type { Messages } from '@/i18n';

// HistoryPage 中文文案，挂在 history.* 命名空间。
export const historyPageZhCN: Messages = {
  history: {
    eyebrow: 'XK-Pane',
    title: '本地历史',
    project: '项目',
    refresh: '刷新',
    noProjects: '尚无已登记项目。请先在侧边栏添加一个项目。',
    snapshotPlaceholder: '项目内相对路径（如 src/main.go），回车快照',
    snapshotAction: '快照',
    recent: '最近变更（{{count}}）',
    empty: '暂无历史记录。在上方为某个文件做一次快照吧。',
    restore: '回滚到此版本',
    snapshot: {
      title: '快照',
      created: '已为该文件保存一个新版本。',
      noChange: '内容未变化，已跳过（与最新版本相同）。',
    },
    restoreConfirm: {
      title: '回滚文件',
      message: '确定把「{{path}}」回滚到该版本吗？将覆盖当前工作树文件（回滚前会自动打标签便于撤销）。',
      ok: '回滚',
      cancel: '取消',
    },
    restored: {
      title: '回滚完成',
      message: '文件已恢复到所选版本。',
    },
    diff: {
      title: '差异：{{path}}（该版本 ↔ 当前工作树）',
      loading: '正在计算差异…',
      binary: '二进制文件，无法显示差异。',
      truncated: '文件过大，差异已截断。',
      identical: '与当前工作树文件无差异。',
      added: '新增',
      removed: '删除',
    },
    config: {
      title: '历史配置与清理',
      enabled: '启用本地历史',
      maxVersionsPerFile: '每文件最多版本数',
      maxAgeDays: '保留天数',
      maxFileSizeMB: '单文件上限（MB）',
      maxTotalSizeMB: '总存储上限（MB）',
      minSaveIntervalSecs: '最小保存间隔（秒）',
      ignorePatterns: '忽略规则（每行一条 glob）',
      save: '保存配置',
      saved: {
        title: '已保存',
        message: '本地历史配置已更新。',
      },
    },
    stats: {
      title: '存储占用',
      summary: '{{versions}} 个版本 · {{files}} 个文件 · {{size}}',
      limit: '上限 {{limit}}',
    },
    cleanup: {
      action: '立即清理',
      confirm: {
        title: '立即清理历史',
        message: '将按当前配置删除过期 / 超量 / 超总大小的旧版本并回收孤儿 blob，删除后不可恢复。继续吗？',
        ok: '清理',
        cancel: '取消',
      },
      done: {
        title: '清理完成',
        message: '已按配置清理旧版本，下方统计已刷新。',
      },
    },
    error: {
      title: '本地历史操作失败',
    },
  },
};
