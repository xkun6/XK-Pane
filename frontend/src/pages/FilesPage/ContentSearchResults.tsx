import { Box, Typography, useTheme } from '@mui/material';
import { useT } from '@/i18n';
import { FileTypeIcon } from '@/components/FileTypeIcon';
import type { ContentMatch } from '@/services';
import { filesPageStyles } from './FilesPage.styles';

// 全文内容搜索结果列表（替换左栏文件树）。每行：文件类型图标 + 相对路径:行号 + 命中行预览。
export const ContentSearchResults = ({
  results,
  loading,
  searched,
  selectedPath,
  onOpen,
}: {
  results: ContentMatch[];
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

  return (
    <Box sx={styles.contentResults}>
      <Typography sx={styles.contentCount}>
        {t('files.content.count', { n: String(results.length) })}
      </Typography>
      {results.map((m, i) => (
        <Box
          key={`${m.path}:${m.line}:${i}`}
          sx={m.path === selectedPath ? styles.contentRowActive : styles.contentRow}
          onClick={() => onOpen(m)}
          title={`${m.rel}:${m.line}`}
        >
          <Box sx={styles.contentRowHead}>
            <FileTypeIcon name={m.rel} />
            <Typography component="span" sx={styles.contentRel}>
              {m.rel}
            </Typography>
            <Typography component="span" sx={styles.contentLineNo}>
              :{m.line}
            </Typography>
          </Box>
          <Typography sx={styles.contentLineText}>{m.text}</Typography>
        </Box>
      ))}
    </Box>
  );
};
