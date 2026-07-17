// 文件页多标签的 Monaco model 管理：每个打开的文件一个 model（独立 undo 栈），
// 关闭 tab / 切根目录时手动 dispose，防止旧内容残留与内存泄漏。
import * as monaco from 'monaco-editor';

const SCHEME = 'xkfile';

// Windows 反斜杠路径转成稳定的 monaco URI 字符串（@monaco-editor/react 的 path prop 会 Uri.parse 它）。
export const toMonacoPath = (path: string): string => `${SCHEME}:///${path.replace(/\\/g, '/')}`;

export const disposeFileModel = (path: string): void => {
  monaco.editor.getModel(monaco.Uri.parse(toMonacoPath(path)))?.dispose();
};

// 只清本 scheme 的 model，不碰其它 Monaco 使用方（如设置页 JSON 编辑框的 inmemory model）。
export const disposeAllFileModels = (): void => {
  monaco.editor.getModels().forEach((m) => {
    if (m.uri.scheme === SCHEME) m.dispose();
  });
};
