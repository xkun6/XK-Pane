import { useEffect, useState } from 'react';
import { Box, Breadcrumbs, Menu, MenuItem, Typography, useTheme } from '@mui/material';
import NavigateNextRoundedIcon from '@mui/icons-material/NavigateNextRounded';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import { useT } from '@/i18n';
import { FileService, type FileEntry } from '@/services';

// 编辑器顶部面包屑：跟随当前打开的文件，展示从树根到文件的路径段。
// 点任一段弹出该目录的下拉（文件夹优先），可逐级钻入（点文件夹换列、点文件打开）。
// 仿 VS Code breadcrumb 行为；路径用原始分隔符切片（D7：不规范化），仅根比较时归一。
interface FileBreadcrumbsProps {
  rootDir: string | null;
  filePath: string;
  onOpenFile: (path: string) => void;
}

interface Crumb {
  label: string;
  listDir: string; // 点击时下拉列出的目录（文件段 = 其父目录，展示同级）
}

const base = (p: string): string => p.replace(/[\\/]+$/, '').replace(/^.*[\\/]/, '');
const parentOf = (p: string): string => {
  const m = /^(.*)[\\/][^\\/]+[\\/]*$/.exec(p);
  return m && m[1] !== undefined ? m[1] : p;
};
const norm = (p: string): string => p.replace(/[\\/]+$/, '').replace(/\\/g, '/').toLowerCase();

// 由原始文件路径与根目录推导面包屑（全程原始分隔符；仅根比较归一）。
function buildCrumbs(rootDir: string, filePath: string): Crumb[] {
  const crumbs: Crumb[] = [{ label: base(rootDir) || rootDir, listDir: rootDir }];
  const ancestors: string[] = [];
  let d = parentOf(filePath);
  const rootN = norm(rootDir);
  // 从文件父目录向上收集到（不含）根目录的各级目录。
  for (let guard = 0; guard < 64 && d && norm(d) !== rootN && norm(d).startsWith(rootN + '/'); guard++) {
    ancestors.unshift(d);
    const up = parentOf(d);
    if (up === d) break;
    d = up;
  }
  for (const a of ancestors) crumbs.push({ label: base(a), listDir: a });
  crumbs.push({ label: base(filePath), listDir: parentOf(filePath) });
  return crumbs;
}

export const FileBreadcrumbs = ({ rootDir, filePath, onOpenFile }: FileBreadcrumbsProps) => {
  const theme = useTheme();
  const fp = theme.palette.foundation;
  const t = useT();
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const [menuDir, setMenuDir] = useState<string | null>(null);
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);

  // 打开 / 切换下拉目录时拉取其直接子项。
  useEffect(() => {
    if (!menuDir) return;
    let alive = true;
    setLoading(true);
    void FileService.listDir(menuDir)
      .then((rows) => {
        if (alive) setEntries(rows);
      })
      .catch(() => {
        if (alive) setEntries([]);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [menuDir]);

  if (!rootDir || norm(filePath).indexOf(norm(rootDir)) !== 0) {
    // 文件不在当前根下（极少见）：退化为只显示文件名，不做下拉。
    return (
      <Typography sx={{ fontSize: 13, color: fp.text.secondary }} title={filePath}>
        {base(filePath)}
      </Typography>
    );
  }

  const crumbs = buildCrumbs(rootDir, filePath);

  const openMenu = (e: React.MouseEvent<HTMLElement>, dir: string): void => {
    setAnchorEl(e.currentTarget);
    setMenuDir(dir);
  };
  const closeMenu = (): void => {
    setAnchorEl(null);
    setMenuDir(null);
  };

  return (
    <>
      <Breadcrumbs
        separator={<NavigateNextRoundedIcon sx={{ fontSize: 14, color: fp.text.muted }} />}
        maxItems={6}
        aria-label={t('files.breadcrumb.ariaLabel')}
        sx={{ minWidth: 0, overflow: 'hidden', '& .MuiBreadcrumbs-ol': { flexWrap: 'nowrap' } }}
      >
        {crumbs.map((c, i) => (
          <Box
            key={`${c.listDir}/${c.label}/${i}`}
            component="button"
            type="button"
            onClick={(e) => openMenu(e, c.listDir)}
            sx={{
              border: 'none',
              background: 'none',
              p: 0,
              cursor: 'pointer',
              fontSize: 13,
              fontFamily: 'inherit',
              whiteSpace: 'nowrap',
              color: i === crumbs.length - 1 ? fp.text.primary : fp.text.secondary,
              '&:hover': { color: fp.accent },
            }}
          >
            {c.label}
          </Box>
        ))}
      </Breadcrumbs>

      <Menu
        open={anchorEl !== null}
        anchorEl={anchorEl}
        onClose={closeMenu}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
        slotProps={{
          paper: {
            sx: {
              borderRadius: 8,
              minWidth: 200,
              maxHeight: 360,
              backgroundColor: fp.bg.elevated,
              border: `1px solid ${fp.divider}`,
            },
          },
        }}
      >
        {loading ? (
          <MenuItem disabled sx={{ fontSize: 13 }}>
            {t('files.tree.loading')}
          </MenuItem>
        ) : entries.length === 0 ? (
          <MenuItem disabled sx={{ fontSize: 13 }}>
            {t('files.tree.empty')}
          </MenuItem>
        ) : (
          entries.map((entry) =>
            entry.isDir ? (
              <MenuItem
                key={entry.path}
                onClick={() => setMenuDir(entry.path)}
                sx={{ fontSize: 13, gap: 0.75, color: fp.text.primary }}
              >
                <FolderRoundedIcon fontSize="small" sx={{ color: fp.accent }} />
                {entry.name}
              </MenuItem>
            ) : (
              <MenuItem
                key={entry.path}
                onClick={() => {
                  onOpenFile(entry.path);
                  closeMenu();
                }}
                selected={norm(entry.path) === norm(filePath)}
                sx={{ fontSize: 13, gap: 0.75, color: fp.text.primary }}
              >
                <DescriptionRoundedIcon fontSize="small" sx={{ color: fp.text.secondary }} />
                {entry.name}
              </MenuItem>
            ),
          )
        )}
      </Menu>
    </>
  );
};
