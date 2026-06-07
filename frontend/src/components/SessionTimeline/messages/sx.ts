import type { SxProps, Theme } from '@mui/material';

// mergeSx：合并若干 sx（过滤掉 false/undefined 落选项）。styles 工厂返回纯对象，数组形态由 MUI 运行时支持，
// 这里断言以过类型。SessionTimeline 卡片们共用（替代各文件各拷一份的本地 merge/sx）。
export const mergeSx = (...parts: Array<SxProps<Theme> | false | undefined>): SxProps<Theme> =>
  parts.filter(Boolean) as SxProps<Theme>;
