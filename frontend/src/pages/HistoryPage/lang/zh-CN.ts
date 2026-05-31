import type { Messages } from '@/i18n';

// HistoryPage 中文文案，挂在 history.* 命名空间。恢复自旧 HistoryPage（d44625c^）并扩展为 4-tab。
export const historyPageZhCN: Messages = {
  history: {
    eyebrow: 'XK-Pane',
    title: '本地历史',
    project: '项目',
    refresh: '刷新',
    noProjects: '尚无已登记项目。请先在侧边栏添加一个项目。',
    error: { title: '本地历史操作失败' },

    // 阶段 1：最近变更文件列表
    fileList: {
      loading: '正在加载文件列表…',
      empty: '该项目暂无历史记录。开启自动捕获或手动快照后，文件变更会出现在这里。',
      autoTrack: '保存文件后会自动记录版本',
    },

    // 阶段 2：版本时间线 + 预览
    backToList: '返回文件列表',
    timeline: {
      title: '版本',
      selectVersion: '从左侧选择一个版本查看。',
      empty: '该文件暂无版本记录。',
      current: '工作树当前',
      rightClickHint: '右键版本可打标签',
    },
    tabs: {
      diff: '差异',
      content: '内容',
      deleted: '已删除',
      projectRestore: '项目快照',
    },
    branchFilter: {
      all: '全部分支',
      unknown: '未知分支',
      label: '分支',
    },

    // diff tab
    diff: {
      loading: '正在计算差异…',
      binary: '二进制文件，无法显示差异。',
      truncated: '文件过大，差异已截断。',
      identical: '与对比目标无差异。',
      added: '新增',
      removed: '删除',
      vsWorking: '该版本 ↔ 当前工作树',
      betweenVersions: '上一版本 ↔ 该版本',
    },

    // content tab
    content: {
      loading: '正在加载内容…',
      empty: '（空文件）',
    },

    // deleted tab
    deleted: {
      empty: '没有已删除的文件。',
      restore: '恢复此文件',
      restoreConfirm: {
        title: '恢复已删除文件',
        message: '确定恢复「{{path}}」到删除前的最后版本吗？将写回工作树。',
        ok: '恢复',
        cancel: '取消',
      },
      restored: { title: '已恢复', message: '文件已恢复到工作树。' },
    },

    // project-restore tab
    projectRestore: {
      empty: '暂无项目快照 / 标签。',
      restoreTo: '恢复到此快照',
      fileCount: '{{count}} 个文件',
      confirm: {
        title: '恢复到快照',
        message:
          '确定把项目恢复到「{{name}}」（{{count}} 个文件）吗？将覆盖这些文件的当前工作树内容（恢复前会自动打标签便于撤销）。',
        ok: '恢复',
        cancel: '取消',
      },
      restored: { title: '恢复完成', message: '已恢复 {{count}} 个文件到工作树。' },
      source: {
        git_commit: 'Git 提交',
        claude_session: 'Claude 会话',
        user: '手动',
        build: '构建',
        restore: '恢复点',
        branch_switch: '分支切换',
        auto: '自动',
        manual: '手动',
      },
    },

    // 打标签
    tag: {
      add: '打标签',
      title: '为该版本打标签',
      label: '标签名',
      placeholder: '如：发布前 / 重要里程碑',
      ok: '保存标签',
      added: { title: '已打标签', message: '标签已保存。' },
      failed: '打标签失败',
    },

    // 操作区：手动快照
    snapshotPlaceholder: '项目内相对路径（如 src/main.go），回车快照',
    snapshotAction: '快照',
    snapshot: {
      title: '快照',
      created: '已为该文件保存一个新版本。',
      noChange: '内容未变化，已跳过（与最新版本相同）。',
    },

    // 单文件回滚
    restore: '回滚到此版本',
    restoreConfirm: {
      title: '回滚文件',
      message: '确定把「{{path}}」回滚到该版本吗？将覆盖当前工作树文件（回滚前会自动打标签便于撤销）。',
      ok: '回滚',
      cancel: '取消',
    },
    restored: { title: '回滚完成', message: '文件已恢复到所选版本。' },

    // 配置 / 统计 / 清理
    config: {
      title: '历史配置与清理',
      enabled: '启用本地历史（自动捕获）',
      maxVersionsPerFile: '每文件最多版本数',
      maxAgeDays: '保留天数',
      maxFileSizeMB: '单文件上限（MB）',
      maxTotalSizeMB: '总存储上限（MB）',
      minSaveIntervalSecs: '最小保存间隔（秒）',
      ignorePatterns: '忽略规则（每行一条 glob）',
      save: '保存配置',
      saved: { title: '已保存', message: '本地历史配置已更新。' },
    },
    stats: {
      title: '存储占用',
      summary: '{{versions}} 个版本 · {{files}} 个文件 · {{size}}',
    },
    cleanup: {
      action: '立即清理',
      confirm: {
        title: '立即清理历史',
        message: '将按当前配置删除过期 / 超量 / 超总大小的旧版本并回收孤儿 blob，删除后不可恢复。继续吗？',
        ok: '清理',
        cancel: '取消',
      },
      done: { title: '清理完成', message: '已按配置清理旧版本，下方统计已刷新。' },
    },
  },
};
