import { Box, useTheme } from '@mui/material';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

export interface MarkdownViewProps {
  source: string;
}

// Markdown 渲染（GFM：表格 / 删除线 / 任务列表）。配色全取 foundation palette。
// react-markdown 默认净化 HTML（不渲染原始 <script>），无需额外 sanitize。
export const MarkdownView = ({ source }: MarkdownViewProps) => {
  const theme = useTheme();
  const fp = theme.palette.foundation;
  return (
    <Box
      sx={{
        flex: 1,
        minHeight: 0,
        minWidth: 0,
        overflowY: 'auto',
        p: 2.5,
        color: fp.text.primary,
        fontSize: 14,
        lineHeight: 1.7,
        '& h1, & h2, & h3, & h4': { fontWeight: 700, mt: 2, mb: 1, color: fp.text.primary },
        '& h1': { fontSize: 26 },
        '& h2': { fontSize: 21 },
        '& h3': { fontSize: 17 },
        '& p': { my: 1 },
        '& a': {
          color: fp.accent,
          textDecoration: 'none',
          '&:hover': { textDecoration: 'underline' },
        },
        '& code': {
          fontFamily: 'monospace',
          fontSize: 12.5,
          backgroundColor: fp.bg.elevated,
          borderRadius: '4px',
          px: 0.5,
          py: 0.25,
        },
        '& pre': {
          backgroundColor: fp.bg.elevated,
          borderRadius: 2,
          p: 1.5,
          overflowX: 'auto',
          '& code': { backgroundColor: 'transparent', p: 0, fontSize: 12.5 },
        },
        '& blockquote': {
          borderLeft: `3px solid ${fp.divider}`,
          pl: 2,
          my: 1,
          color: fp.text.secondary,
        },
        '& ul, & ol': { pl: 3, my: 1 },
        '& table': { borderCollapse: 'collapse', my: 1.5, fontSize: 13 },
        '& th, & td': { border: `1px solid ${fp.divider}`, px: 1, py: 0.5, textAlign: 'left' },
        '& th': { backgroundColor: fp.bg.elevated, fontWeight: 600 },
        '& img': { maxWidth: '100%' },
        '& hr': { border: 'none', borderTop: `1px solid ${fp.divider}`, my: 2 },
      }}
    >
      <Markdown remarkPlugins={[remarkGfm]}>{source}</Markdown>
    </Box>
  );
};
