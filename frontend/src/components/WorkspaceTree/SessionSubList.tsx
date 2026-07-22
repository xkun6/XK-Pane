import { useState, type MouseEvent } from 'react';
import { Box, Divider, ListItemText, Menu, MenuItem } from '@mui/material';
import SmartToyRoundedIcon from '@mui/icons-material/SmartToyRounded';
import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import BoltRoundedIcon from '@mui/icons-material/BoltRounded';
import { useT } from '@/i18n';
import type { ResumeSession } from '@/services';
import type { WorkspaceTreeSx } from './WorkspaceTree.styles';
import type { MenuAnchor } from './WorkspaceContextMenu';
import { useSessionSubList, type SubListTarget } from './useSessionSubList';

export interface SessionSubListProps {
  target: SubListTarget;
  styles: WorkspaceTreeSx;
}

function relTime(modifiedAt: number, t: (k: string, p?: Record<string, string>) => string): string {
  const sec = Math.max(0, Math.floor(Date.now() / 1000 - modifiedAt));
  if (sec < 60) return t('workspaceTree.sessions.time.justNow');
  const min = Math.floor(sec / 60);
  if (min < 60) return t('workspaceTree.sessions.time.minutes', { n: String(min) });
  const hr = Math.floor(min / 60);
  if (hr < 24) return t('workspaceTree.sessions.time.hours', { n: String(hr) });
  return t('workspaceTree.sessions.time.days', { n: String(Math.floor(hr / 24)) });
}

// 工作空间项目节点下的会话子列表（按需扫盘）：左键恢复 / 右键菜单（恢复·以 reclaude 恢复·复制 id）。
// 移植 cc-pane SessionSubList 的 local 子集；引擎用 MUI 图标区分
// （claude=机器人 / codex=星耀 / grok=闪电），不用 cc-pane 的品牌 SVG（Icon 铁律：禁内联 SVG / 第三方）。
export const SessionSubList = ({ target, styles }: SessionSubListProps) => {
  const t = useT();
  const vm = useSessionSubList(target);
  const [menu, setMenu] = useState<{ session: ResumeSession; anchor: MenuAnchor } | null>(null);

  const openMenu = (session: ResumeSession, e: MouseEvent): void => {
    e.preventDefault();
    e.stopPropagation();
    setMenu({ session, anchor: { top: e.clientY, left: e.clientX } });
  };
  const closeMenu = (): void => setMenu(null);

  if (vm.loading) {
    return <Box sx={styles.sessionList}><Box sx={styles.sessionHint}>{t('workspaceTree.sessions.loading')}</Box></Box>;
  }
  if (vm.sessions.length === 0) {
    return <Box sx={styles.sessionList}><Box sx={styles.sessionHint}>{t('workspaceTree.sessions.empty')}</Box></Box>;
  }

  return (
    <Box sx={styles.sessionList}>
      {vm.sessions.map((s) => (
        <Box
          key={s.filePath}
          sx={styles.sessionRow}
          role="button"
          tabIndex={0}
          title={s.description || s.id}
          onClick={() => vm.resume(s)}
          onContextMenu={(e) => openMenu(s, e)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              vm.resume(s);
            }
          }}
        >
          {s.source === 'claude' ? (
            <SmartToyRoundedIcon sx={{ ...styles.sessionGlyph, fontSize: 14 }} />
          ) : s.source === 'grok' ? (
            <BoltRoundedIcon sx={{ ...styles.sessionGlyph, fontSize: 14 }} />
          ) : (
            <AutoAwesomeRoundedIcon sx={{ ...styles.sessionGlyph, fontSize: 14 }} />
          )}
          <Box sx={styles.sessionDesc}>{s.description || t('workspaceTree.sessions.noPrompt')}</Box>
          <Box component="span" sx={styles.sessionTime}>
            {relTime(s.modifiedAt, t)}
          </Box>
        </Box>
      ))}

      <Menu
        open={menu !== null}
        onClose={closeMenu}
        anchorReference="anchorPosition"
        anchorPosition={menu ? { top: menu.anchor.top, left: menu.anchor.left } : undefined}
      >
        <MenuItem
          onClick={() => {
            if (menu) vm.resume(menu.session);
            closeMenu();
          }}
        >
          <ListItemText>{t('workspaceTree.sessions.resume')}</ListItemText>
        </MenuItem>
        {menu?.session.source === 'claude' && (
          <MenuItem
            onClick={() => {
              if (menu) vm.resume(menu.session, 'reclaude');
              closeMenu();
            }}
          >
            <ListItemText>{t('workspaceTree.sessions.resumeWithReclaude')}</ListItemText>
          </MenuItem>
        )}
        <Divider />
        <MenuItem
          onClick={() => {
            if (menu) vm.copyId(menu.session);
            closeMenu();
          }}
        >
          <ListItemText>{t('workspaceTree.sessions.copyId')}</ListItemText>
        </MenuItem>
      </Menu>
    </Box>
  );
};
