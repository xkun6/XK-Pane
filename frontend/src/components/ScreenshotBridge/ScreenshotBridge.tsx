import { Alert, Snackbar } from '@mui/material';
import { useT } from '@/i18n';
import { useScreenshotBridge } from './useScreenshotBridge';

// 截图闭环桥（无界面常驻组件）：业务在 useScreenshotBridge（订阅 screenshot:saved →
// 路径写活跃终端），这里仅渲染结果 snackbar（局部 Snackbar 范式，同 WebWorkspaceTree）。
export const ScreenshotBridge = () => {
  const vm = useScreenshotBridge();
  const t = useT();

  return (
    <Snackbar
      open={vm.notice !== null}
      autoHideDuration={2400}
      onClose={vm.clearNotice}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
    >
      {vm.notice ? (
        <Alert severity="success" variant="filled" onClose={vm.clearNotice}>
          {t(`screenshotBridge.notice.${vm.notice}`)}
        </Alert>
      ) : (
        <span />
      )}
    </Snackbar>
  );
};
