import type { ReactNode } from 'react';
import { Box, Button, Typography, useTheme } from '@mui/material';
import type { SvgIconComponent } from '@mui/icons-material';
import ChecklistRoundedIcon from '@mui/icons-material/ChecklistRounded';
import TagRoundedIcon from '@mui/icons-material/TagRounded';
import WbSunnyRoundedIcon from '@mui/icons-material/WbSunnyRounded';
import PublicRoundedIcon from '@mui/icons-material/PublicRounded';
import AccountTreeRoundedIcon from '@mui/icons-material/AccountTreeRounded';
import CallSplitRoundedIcon from '@mui/icons-material/CallSplitRounded';
import OpenInNewRoundedIcon from '@mui/icons-material/OpenInNewRounded';
import TerminalRoundedIcon from '@mui/icons-material/TerminalRounded';
import { useT } from '@/i18n';
import { todoPageStyles } from '../TodoPage.styles';
import type { ViewMode } from '../useTodoPage';

interface Props {
  viewMode: ViewMode;
  activeScope: string | null;
  onViewModeChange: (m: ViewMode) => void;
  onScopeChange: (s: string | null) => void;
}

// 作用域导航项（值 / 图标 / i18n key），对齐 cc-pane TodoSidebar 作用域段。
const SCOPES: { value: string; icon: SvgIconComponent; labelKey: string }[] = [
  { value: 'global', icon: PublicRoundedIcon, labelKey: 'todo.scope.global' },
  { value: 'workspace', icon: AccountTreeRoundedIcon, labelKey: 'todo.scope.workspace' },
  { value: 'project', icon: CallSplitRoundedIcon, labelKey: 'todo.scope.project' },
  { value: 'external', icon: OpenInNewRoundedIcon, labelKey: 'todo.scope.external' },
  { value: 'temp_script', icon: TerminalRoundedIcon, labelKey: 'todo.scope.temp_script' },
];

export const TodoSidebar = ({ viewMode, activeScope, onViewModeChange, onScopeChange }: Props) => {
  const theme = useTheme();
  const styles = todoPageStyles(theme);
  const t = useT();

  const navItem = (icon: SvgIconComponent, label: string, active: boolean, onClick: () => void): ReactNode => {
    const Icon = icon;
    return (
      <Button
        key={label}
        disableRipple
        sx={active ? styles.navItemActive : styles.navItem}
        startIcon={<Icon fontSize="small" />}
        onClick={onClick}
      >
        <Box component="span" sx={styles.navLabel}>
          {label}
        </Box>
      </Button>
    );
  };

  return (
    <Box sx={styles.sidebar}>
      <Box sx={styles.sidebarBrand}>
        <ChecklistRoundedIcon fontSize="small" />
        <span>{t('todo.brand')}</span>
      </Box>
      <Box component="nav" sx={styles.sidebarNav}>
        <Typography sx={styles.sectionLabel}>{t('todo.sidebar.mainViews')}</Typography>
        {navItem(TagRoundedIcon, t('todo.sidebar.allTasks'), viewMode === 'all' && activeScope === null, () => {
          onViewModeChange('all');
          onScopeChange(null);
        })}
        {navItem(WbSunnyRoundedIcon, t('todo.sidebar.myDay'), viewMode === 'my_day', () => onViewModeChange('my_day'))}

        <Typography sx={styles.sectionLabel}>{t('todo.sidebar.scopes')}</Typography>
        {SCOPES.map((s) =>
          navItem(s.icon, t(s.labelKey), viewMode === 'all' && activeScope === s.value, () => {
            onViewModeChange('all');
            onScopeChange(s.value);
          }),
        )}
      </Box>
    </Box>
  );
};
