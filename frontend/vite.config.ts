import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
      "@bindings": path.resolve(__dirname, "bindings"),
    },
  },
  // 冷启动竞态修复：dev 模式下 wails3 dev 把 Vite（common:dev:frontend）以 background 启动且不等
  // 它就绪，窗口（wails3 task run）随即开窗加载 9245。首次 .vite 缓存为冷时，Vite 会在首个模块
  // 请求时才**逐个发现**依赖（尤其 @mui/icons-material/*Rounded 几十个深路径导入）→ 触发
  // 「optimized deps changed, reloading」整页重载竞态 → main.tsx 模块图卡住、React 不挂载、骨架
  // 不消（表现为首次 dev 卡启动骨架、第二次才正常 = 第二次 .vite 已热）。把运行时依赖列入
  // optimizeDeps.include，让 Vite 在 server 启动时一次性预打包，避免首个请求期的逐个发现+重载。
  optimizeDeps: {
    include: [
      "react",
      "react-dom",
      "react-dom/client",
      "@emotion/react",
      "@emotion/styled",
      "@mui/material",
      "@mui/system",
      "@mui/icons-material",
      "@mui/x-charts",
      "@mui/x-data-grid",
      "@xterm/xterm",
      "@xterm/addon-fit",
      "qrcode.react",
      "@dnd-kit/core",
      "@dnd-kit/sortable",
      "@dnd-kit/utilities",
      "@monaco-editor/react",
      "monaco-editor",
      "react-markdown",
      "remark-gfm",
    ],
  },
  build: {
    rollupOptions: {
      input: {
        main: path.resolve(__dirname, "index.html"),
        child: path.resolve(__dirname, "child.html"),
      },
    },
  },
  plugins: [react(), wails("./bindings")],
});
