import type { Messages } from '@/i18n';

export const screenshotZhCN: Messages = {
  settings: {
    screenshot: {
      title: '截图快捷键',
      hint: '在任意界面按下快捷键即可框选截取屏幕区域。PNG 保存后路径自动粘贴到活动终端（同时复制到剪贴板），供 Claude Code 引用图片。仅 Windows 生效。',
      shortcut: {
        label: '快捷键',
        recording: '按下新的组合键（Esc 退出）…',
        hint: '至少一个修饰键（Ctrl / Alt / Shift / Win）+ 一个主键（A-Z / 0-9 / F1-F24）。与其它应用冲突时会提示失败。',
        apply: '应用',
        cancel: '取消',
        ariaRecorder: '快捷键录制框',
      },
      retention: {
        label: '保留天数',
        hint: '每次保存截图时自动清理早于保留天数的旧截图；设为 0 则永久保留。',
      },
      feedback: {
        applying: '正在更新快捷键…',
        applied: '快捷键已更新',
        failed: '快捷键更新失败：{{message}}',
      },
    },
  },
};
