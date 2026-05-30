import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';

const container = document.getElementById('root');
if (!container) {
  throw new Error('Root element #root not found');
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);

// 禁用 webview 默认右键菜单（桌面应用不该出现「返回 / 刷新 / 另存为 / 检查」等浏览器菜单）。
// 例外：可编辑元素（input / textarea / contenteditable）保留系统复制 / 粘贴菜单。
// 应用内自定义右键菜单由各组件 onContextMenu + preventDefault 弹出，不受此影响。
document.addEventListener('contextmenu', (e) => {
  const target = e.target as HTMLElement | null;
  if (target?.closest('input, textarea, [contenteditable="true"]')) return;
  e.preventDefault();
});

// 启动骨架淡出与移除：
// 在 React 完成首次提交、首帧绘制完成之后再淡出，避免"骨架消失 → 真实 UI 出现"之间的白闪。
//
// 时序：
//   1) createRoot.render() 调度首次渲染
//   2) requestAnimationFrame 等首帧绘制完成
//   3) 给骨架加 fade-out class（CSS transition 160ms 完成淡出）
//   4) transition 结束后 remove 节点（同时 remove 监听器）
//
// 设计兜底：哪怕浏览器掐了 transitionend，800ms 后强行移除骨架（避免长留 DOM）。
const removeStartupSkeleton = (): void => {
  const skeleton = document.getElementById('startup-skeleton');
  if (!skeleton) return;

  let removed = false;
  const remove = (): void => {
    if (removed) return;
    removed = true;
    skeleton.parentNode?.removeChild(skeleton);
  };

  skeleton.addEventListener('transitionend', remove, { once: true });
  // 兜底：transition 没触发也 800ms 后移除
  window.setTimeout(remove, 800);

  skeleton.classList.add('fade-out');
};

// 等首帧绘制完再淡出 —— rAF 嵌套两层确保 React 已完成首次 commit + paint
requestAnimationFrame(() => {
  requestAnimationFrame(removeStartupSkeleton);
});
