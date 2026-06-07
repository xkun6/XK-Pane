import { useEffect, useState } from 'react';
import { IconButton, Tooltip } from '@mui/material';
import type { SxProps, Theme } from '@mui/material';
import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import CheckRoundedIcon from '@mui/icons-material/CheckRounded';
import { useT } from '@/i18n';
import { copyText } from '@/shared/clipboard';

// CopyButton：把传入文本写进剪贴板（走 shared/clipboard，兼容 Web 镜像非安全上下文），成功后图标短暂变勾。
// 显隐由父卡片样式经 .xk-copy-btn 控制（hover 显示 / touch 常驻）。空文本不渲染（不给空复制入口）。
export function CopyButton({ text, sx }: { text: string; sx?: SxProps<Theme> }) {
  const t = useT();
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const id = setTimeout(() => setCopied(false), 1500);
    return () => clearTimeout(id);
  }, [copied]);
  if (!text) return null; // hooks 已全部调用，安全 return
  const onCopy = async (e: React.MouseEvent): Promise<void> => {
    e.stopPropagation(); // 折叠卡 header 上点复制不触发展开 / 折叠
    if (await copyText(text)) setCopied(true);
  };
  return (
    <Tooltip title={copied ? t('sessionTimeline.copied') : t('sessionTimeline.copy')}>
      <IconButton
        className="xk-copy-btn"
        size="small"
        onClick={(e) => void onCopy(e)}
        aria-label={t('sessionTimeline.copy')}
        sx={sx}
      >
        {copied ? <CheckRoundedIcon fontSize="inherit" /> : <ContentCopyRoundedIcon fontSize="inherit" />}
      </IconButton>
    </Tooltip>
  );
}
