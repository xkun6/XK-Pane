import { type ReactNode } from 'react';
import { Box, Typography, useTheme, type SxProps, type Theme } from '@mui/material';
import { useT } from '@/i18n';
import { FileTypeIcon } from '@/components/FileTypeIcon';
import type { ContentMatch } from '@/services';
import { filesPageStyles } from './FilesPage.styles';

// 全文内容搜索结果（仿 VS Code 按文件分组）：每个文件一个头（图标 + 文件名 + 压暗目录 + 命中数），
// 其下嵌套各命中行（行号 + 预览，命中词高亮）。避免旧版逐行重复整条相对路径 + rtl 截断的杂乱观感。
const baseOf = (rel: string): string => rel.replace(/\/+$/, '').replace(/^.*\//, '');
const dirOf = (rel: string): string => {
  const i = rel.replace(/\/+$/, '').lastIndexOf('/');
  return i >= 0 ? rel.slice(0, i) : '';
};

interface FileGroup {
  path: string;
  rel: string;
  hits: ContentMatch[];
}

// 把命中行预览按 term（不区分大小写、子串）切成普通段 / 高亮段。term 为空原样返回。
function highlight(text: string, term: string, hlSx: SxProps<Theme>): ReactNode {
  const q = term.toLowerCase();
  if (!q) return text;
  const lower = text.toLowerCase();
  const out: ReactNode[] = [];
  let i = 0;
  let k = 0;
  while (i < text.length) {
    const at = lower.indexOf(q, i);
    if (at < 0) {
      out.push(text.slice(i));
      break;
    }
    if (at > i) out.push(text.slice(i, at));
    out.push(
      <Box component="span" key={k++} sx={hlSx}>
        {text.slice(at, at + q.length)}
      </Box>,
    );
    i = at + q.length;
  }
  return out;
}

export const ContentSearchResults = ({
  results,
  term,
  loading,
  searched,
  selectedPath,
  onOpen,
}: {
  results: ContentMatch[];
  term: string;
  loading: boolean;
  searched: boolean;
  selectedPath: string | null;
  onOpen: (m: ContentMatch) => void;
}) => {
  const theme = useTheme();
  const styles = filesPageStyles(theme);
  const t = useT();

  if (loading) return <Box sx={styles.searchHintBox}>{t('files.content.searching')}</Box>;
  if (!searched) return <Box sx={styles.searchHintBox}>{t('files.content.hint')}</Box>;
  if (results.length === 0) return <Box sx={styles.searchHintBox}>{t('files.content.noMatch')}</Box>;

  // 按文件聚合，保持首次出现顺序。
  const groups: FileGroup[] = [];
  const byPath = new Map<string, FileGroup>();
  for (const m of results) {
    let g = byPath.get(m.path);
    if (!g) {
      g = { path: m.path, rel: m.rel, hits: [] };
      byPath.set(m.path, g);
      groups.push(g);
    }
    g.hits.push(m);
  }

  return (
    <Box sx={styles.contentResults}>
      <Typography sx={styles.contentCount}>
        {t('files.content.count', { n: String(results.length) })}
      </Typography>
      {groups.map((g) => {
        const fileName = baseOf(g.rel);
        const dir = dirOf(g.rel);
        const active = g.path === selectedPath;
        return (
          <Box key={g.path} sx={styles.contentGroup}>
            <Box
              sx={active ? styles.contentFileActive : styles.contentFile}
              onClick={() => g.hits[0] && onOpen(g.hits[0])}
              title={g.rel}
            >
              <FileTypeIcon name={g.rel} />
              <Typography component="span" sx={styles.contentFileName}>
                {fileName}
              </Typography>
              {dir && (
                <Typography component="span" sx={styles.contentFileDir}>
                  {dir}
                </Typography>
              )}
              <Box component="span" sx={styles.contentHitCount}>
                {g.hits.length}
              </Box>
            </Box>
            {g.hits.map((m, i) => (
              <Box
                key={`${m.line}:${i}`}
                sx={styles.contentHit}
                onClick={() => onOpen(m)}
                title={`${m.rel}:${m.line}`}
              >
                <Box component="span" sx={styles.contentHitLine}>
                  {m.line}
                </Box>
                <Typography component="span" sx={styles.contentHitText}>
                  {highlight(m.text, term, styles.contentHitHL as SxProps<Theme>)}
                </Typography>
              </Box>
            ))}
          </Box>
        );
      })}
    </Box>
  );
};
