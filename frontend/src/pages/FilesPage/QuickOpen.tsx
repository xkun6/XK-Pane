import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Box, InputBase, Modal, Typography, useTheme, type SxProps, type Theme } from '@mui/material';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import { useT } from '@/i18n';
import { FileService, type ListedFile } from '@/services';

// Ctrl+P 快速打开：仿 VS Code 命令面板的文件模糊检索浮层。
// 打开时一次性拉取 root 下的文件清单（后端 ListFiles 双预算截断），按子序列模糊打分排序；
// 支持 "路径:行号" 语法跳转到指定行。仅 FilesPage 使用（Ctrl+P 限定在文件页内，避让终端）。
interface QuickOpenProps {
  open: boolean;
  rootPath: string | null;
  onClose: () => void;
  onPick: (path: string, line?: number) => void;
}

const MAX_SHOWN = 50;

// 解析查询尾部的 ":行号"（如 "foo.ts:42"）。
function parseQuery(raw: string): { query: string; line?: number } {
  const m = /^(.*?):(\d+)\s*$/.exec(raw);
  if (m && m[1] !== undefined) return { query: m[1].trim(), line: Number(m[2]) };
  return { query: raw.trim() };
}

// 子序列模糊打分：返回 { score, indices } 或 null（不匹配）。
// 命中文件名段加权、连续命中加权、文件名首字符额外加权；短路径轻微优先。
function fuzzyScore(rel: string, q: string): { score: number; indices: number[] } | null {
  const text = rel.toLowerCase();
  const baseStart = rel.lastIndexOf('/') + 1;
  const indices: number[] = [];
  let score = 0;
  let qi = 0;
  let prev = -2;
  for (let i = 0; i < text.length && qi < q.length; i++) {
    if (text[i] === q[qi]) {
      indices.push(i);
      let s = 1;
      if (i === prev + 1) s += 4;
      if (i >= baseStart) s += 3;
      if (i === baseStart) s += 5;
      score += s;
      prev = i;
      qi += 1;
    }
  }
  if (qi < q.length) return null;
  return { score: score - rel.length * 0.05, indices };
}

// 把 rel 单趟渲染为按样式分组的 span：命中字符 → 高亮，目录前缀的非命中字符 → 压暗，
// 文件名非命中字符 → 主色。连续同样式字符合并为一段，减少节点数。
function renderRel(rel: string, indices: number[], hlSx: SxProps<Theme>, dimSx: SxProps<Theme>): ReactNode {
  const baseStart = rel.lastIndexOf('/') + 1;
  const hit = new Set(indices);
  const styleAt = (i: number): 'hl' | 'dim' | 'plain' => (hit.has(i) ? 'hl' : i < baseStart ? 'dim' : 'plain');
  const out: ReactNode[] = [];
  let buf = '';
  let cur = styleAt(0);
  let start = 0;
  const sxOf = (s: 'hl' | 'dim' | 'plain'): SxProps<Theme> | undefined => (s === 'hl' ? hlSx : s === 'dim' ? dimSx : undefined);
  const flush = () => {
    if (buf === '') return;
    out.push(
      <Box component="span" key={start} sx={sxOf(cur)}>
        {buf}
      </Box>,
    );
    buf = '';
  };
  for (let i = 0; i < rel.length; i++) {
    const s = styleAt(i);
    if (s !== cur) {
      flush();
      cur = s;
      start = i;
    }
    buf += rel[i];
  }
  flush();
  return out;
}

export const QuickOpen = ({ open, rootPath, onClose, onPick }: QuickOpenProps) => {
  const theme = useTheme();
  const fp = theme.palette.foundation;
  const t = useT();
  const [files, setFiles] = useState<ListedFile[]>([]);
  const [raw, setRaw] = useState('');
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLDivElement | null>(null);
  const rowRefs = useRef<Map<number, HTMLDivElement>>(new Map());

  // 打开时拉取文件清单并重置查询；关闭时清空缓存（下次打开拉最新）。
  useEffect(() => {
    if (!open || !rootPath) return;
    setRaw('');
    setActive(0);
    let alive = true;
    void FileService.listFiles(rootPath)
      .then((rows) => {
        if (alive) setFiles(rows);
      })
      .catch(() => {
        if (alive) setFiles([]);
      });
    return () => {
      alive = false;
    };
  }, [open, rootPath]);

  const { query, line } = parseQuery(raw);

  const matches = useMemo(() => {
    if (query === '') return files.slice(0, MAX_SHOWN).map((f) => ({ file: f, indices: [] as number[] }));
    const lower = query.toLowerCase();
    const scored: { file: ListedFile; indices: number[]; score: number }[] = [];
    for (const f of files) {
      const r = fuzzyScore(f.rel, lower);
      if (r) scored.push({ file: f, indices: r.indices, score: r.score });
    }
    scored.sort((a, b) => b.score - a.score || a.file.rel.length - b.file.rel.length);
    return scored.slice(0, MAX_SHOWN);
  }, [files, query]);

  // 命中集合变化后把游标夹到范围内。
  useEffect(() => {
    setActive((i) => (i >= matches.length ? Math.max(0, matches.length - 1) : i));
  }, [matches.length]);

  // 游标变化滚动入视。
  useEffect(() => {
    rowRefs.current.get(active)?.scrollIntoView({ block: 'nearest' });
  }, [active]);

  const pick = (idx: number): void => {
    const hit = matches[idx];
    if (!hit) return;
    onPick(hit.file.path, line);
    onClose();
  };

  const onKeyDown = (e: React.KeyboardEvent): void => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((i) => Math.min(i + 1, matches.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      pick(active);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
    }
  };

  const hlSx: SxProps<Theme> = { color: fp.accent, fontWeight: 700 };
  const dimSx: SxProps<Theme> = { color: fp.text.muted };

  return (
    <Modal open={open} onClose={onClose} aria-label={t('files.quickOpen.placeholder')}>
      <Box
        onKeyDown={onKeyDown}
        sx={{
          position: 'absolute',
          top: '12%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: 'min(640px, 86vw)',
          maxHeight: '70vh',
          display: 'flex',
          flexDirection: 'column',
          borderRadius: 8,
          backgroundColor: fp.bg.elevated,
          border: `1px solid ${fp.divider}`,
          boxShadow: theme.shadows[8],
          overflow: 'hidden',
          outline: 'none',
        }}
      >
        <InputBase
          autoFocus
          value={raw}
          onChange={(e) => {
            setRaw(e.target.value);
            setActive(0);
          }}
          placeholder={t('files.quickOpen.placeholder')}
          sx={{
            px: 1.5,
            py: 1,
            fontSize: 14,
            color: fp.text.primary,
            borderBottom: `1px solid ${fp.divider}`,
          }}
          inputProps={{ 'aria-label': t('files.quickOpen.placeholder') }}
        />
        <Box ref={listRef} sx={{ overflowY: 'auto', py: 0.5 }}>
          {matches.length === 0 ? (
            <Typography sx={{ px: 1.5, py: 1.5, fontSize: 13, color: fp.text.muted }}>
              {query === '' ? t('files.quickOpen.hint') : t('files.quickOpen.noMatch')}
            </Typography>
          ) : (
            matches.map((m, idx) => (
              <Box
                key={m.file.path}
                ref={(el: HTMLDivElement | null) => {
                  if (el) rowRefs.current.set(idx, el);
                  else rowRefs.current.delete(idx);
                }}
                onClick={() => pick(idx)}
                onMouseMove={() => setActive(idx)}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 0.75,
                  px: 1.5,
                  height: 28,
                  cursor: 'pointer',
                  backgroundColor: idx === active ? fp.bg.active : 'transparent',
                }}
              >
                <DescriptionRoundedIcon fontSize="small" sx={{ flexShrink: 0, color: fp.text.secondary }} />
                <Typography
                  component="div"
                  sx={{
                    flex: 1,
                    minWidth: 0,
                    fontSize: 13,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    color: fp.text.primary,
                  }}
                  title={m.file.rel}
                >
                  {renderRel(m.file.rel, m.indices, hlSx, dimSx)}
                </Typography>
              </Box>
            ))
          )}
        </Box>
      </Box>
    </Modal>
  );
};
