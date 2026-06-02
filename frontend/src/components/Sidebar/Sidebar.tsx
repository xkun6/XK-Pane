import HomeRoundedIcon from '@mui/icons-material/HomeRounded';
import { Box, IconButton, Tooltip, Typography, useTheme } from '@mui/material';
import type { ComponentType, ReactElement } from 'react';
import { useT } from '@/i18n';
import { useRouter } from '@/router';
import type { RouteDefinition } from '@/router';
import { usePreferences } from '@/preferences';
import { lifecycleRole } from '@/services/transport';
import { formatShortcut, SETTINGS_SHORTCUT } from '@/shared/keyboardShortcuts';
import { sidebarStyles } from './Sidebar.styles';

type IconComponent = ComponentType<{ fontSize?: 'small' | 'medium' | 'large' | 'inherit' }>;

// 左侧侧边栏（扁平化，宽 64px）：
//  - 顶部：品牌方块（默认 "F"，文案走 i18n key sidebar.brand）+ 短 divider
//  - 主区：路由表中 slot === 'primary' 的条目
//  - 底部：路由表中 slot === 'footer' 的条目（如 Settings）
//  - 选中态：bg.active 底色 + accent 图标 + 左侧 3px accent indicator bar
//  - Tooltip 可在偏好里关闭（aria-label 仍保留，可访问性不掉）
// 不接受 props —— 路由表是唯一事实源，新增页面只改 routes.tsx。
export const Sidebar = () => {
  const theme = useTheme();
  const styles = sidebarStyles(theme);
  const { primary, footer, active, navigate } = useRouter();
  const { preferences } = usePreferences();
  const t = useT();

  // 路由 label 解析：优先 t(labelKey)，否则回落到 label 字面量
  const labelOf = (r: RouteDefinition): string =>
    r.labelKey ? t(r.labelKey) : r.label;

  // 桌面专属页（desktopOnly）在 Web（viewer）端不显示入口 —— 无文件系统访问。
  const visibleHere = (r: RouteDefinition): boolean =>
    !(r.desktopOnly && lifecycleRole === 'viewer');

  // 数字导航的页序（与 useGlobalShortcuts 的 navItems 同口径：可见的主导航页）。
  const navItems = primary.filter(visibleHere);

  // 该条目对应的快捷键提示文本（拼到 tooltip 末尾，便于发现）：设置用 Mod+,，
  // 主导航页用 Mod+序号。组合键是平台技术记号（非可翻译文案），不走 t()。
  const shortcutHint = (r: RouteDefinition): string | null => {
    if (r.id === 'settings') return formatShortcut(SETTINGS_SHORTCUT);
    const i = navItems.findIndex((n) => n.id === r.id);
    return i >= 0 && i < 9 ? formatShortcut({ mods: ['mod'], key: String(i + 1) }) : null;
  };

  const wrapTooltip = (label: string, child: ReactElement): ReactElement => {
    if (!preferences.showTooltip) return child;
    return (
      <Tooltip title={label} placement="right" arrow>
        {child}
      </Tooltip>
    );
  };

  const renderItem = (r: RouteDefinition) => {
    const isActive = r.id === active.id;
    const label = labelOf(r);
    const hint = shortcutHint(r);
    const tip = hint ? `${label}  ·  ${hint}` : label;
    const Icon: IconComponent = r.icon ?? HomeRoundedIcon;
    return (
      <Box key={r.id} sx={styles.itemWrap}>
        {isActive && <Box sx={styles.indicator} aria-hidden />}
        {wrapTooltip(
          tip,
          <IconButton
            onClick={() => navigate(r.id)}
            sx={isActive ? styles.itemActive : styles.item}
            aria-label={label}
            aria-current={isActive ? 'page' : undefined}
          >
            <Icon fontSize="medium" />
          </IconButton>,
        )}
      </Box>
    );
  };

  return (
    <Box component="nav" sx={styles.root} aria-label={t('sidebar.navAriaLabel')}>
      {preferences.showLogo && (
        <>
          <Box sx={styles.brand} aria-hidden>
            <Typography
              component="span"
              sx={{
                fontSize: 'inherit',
                fontWeight: 'inherit',
                letterSpacing: 'inherit',
                color: 'inherit',
              }}
            >
              {t('sidebar.brand')}
            </Typography>
          </Box>
          <Box sx={styles.brandDivider} aria-hidden />
        </>
      )}

      <Box sx={styles.section}>
        {primary.filter(visibleHere).map(renderItem)}
      </Box>

      <Box sx={styles.spacer} />

      <Box sx={styles.section}>
        {footer.filter(visibleHere).map(renderItem)}
      </Box>
    </Box>
  );
};
