import { defineConfig } from 'vitest/config';

// 独立于 vite.config.ts —— 测试只覆盖纯逻辑（reducer / 树操作），用 node 环境，
// 不引入 jsdom / wails 运行时插件。测试文件用相对 import，无需路径别名。
export default defineConfig({
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
});
