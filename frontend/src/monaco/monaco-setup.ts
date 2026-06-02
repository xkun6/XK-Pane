// Monaco 离线接入（Wails 内嵌 assets，无 CDN）：
//   - 用 npm 包内的 monaco 实例喂给 @monaco-editor/react（loader.config({ monaco })），
//     否则它默认从官方 CDN 拉 monaco —— 离线 / 局域网 Web 端会挂。
//   - language worker 经 Vite 的 ?worker import 打成独立 chunk，由 self.MonacoEnvironment.getWorker
//     按语言分发；构建后 worker 文件进 frontend/dist，随 embed.FS 打入二进制。
import * as monaco from 'monaco-editor';
import { loader } from '@monaco-editor/react';
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker';
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker';
import cssWorker from 'monaco-editor/esm/vs/language/css/css.worker?worker';
import htmlWorker from 'monaco-editor/esm/vs/language/html/html.worker?worker';
import tsWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker?worker';

declare global {
  interface Window {
    MonacoEnvironment?: {
      getWorker: (workerId: string, label: string) => Worker;
    };
  }
}

let configured = false;

// 幂等：首个编辑器挂载前调用一次（CodeEditor 模块顶层副作用调用）。
export function setupMonaco(): void {
  if (configured) return;
  configured = true;
  window.MonacoEnvironment = {
    getWorker(_workerId: string, label: string): Worker {
      switch (label) {
        case 'json':
          return new jsonWorker();
        case 'css':
        case 'scss':
        case 'less':
          return new cssWorker();
        case 'html':
        case 'handlebars':
        case 'razor':
          return new htmlWorker();
        case 'typescript':
        case 'javascript':
          return new tsWorker();
        default:
          return new editorWorker();
      }
    },
  };
  loader.config({ monaco });
}
