// OSC 52 剪贴板 provider —— 喂给官方 @xterm/addon-clipboard，替掉原先手写的 OSC 52 解析。
//
// 为什么不能直接用官方的 BrowserClipboardProvider：它调 navigator.clipboard.writeText，而 Wails
// WebView 里该 API 受用户手势限制 —— OSC 52 是终端内的程序主动发起的，没有手势上下文，写入会被
// 静默拒绝（正是当初手写实现改走 Wails 原生 Clipboard 的原因）。故只复用官方的 OSC 52 解析 +
// UTF-8 安全的 base64（它的 Base64 走 TextDecoder，中文不会乱码），写入仍走原生通道。
//
// 两个刻意保留的保守取舍（都与官方 addon 的默认行为不同）：
//   - readText 恒返回空串：不支持 OSC 52 读剪贴板。读 = 把用户剪贴板内容交给终端里运行的程序，
//     原手写实现就刻意忽略读请求（payload '?' 直接返回 null），这里维持不泄露 —— 程序只会得知
//     「剪贴板为空」。
//   - 空文本直接跳过：官方 addon 在数据为空 / base64 解码失败时会拿空串调 writeText，按 OSC 52
//     规范那是「清空剪贴板」。但程序误发一次就把用户剪贴板清了，破坏性不对称，故忽略。
import type { IClipboardProvider } from '@xterm/addon-clipboard';

// writeText: 真正的写入通道（生产上是 Wails 原生 ClipboardService.writeText，它返回
// Promise<boolean> 表示是否写成功，故这里收 Promise<unknown>；与原手写实现一致地忽略该返回值 ——
// 剪贴板写失败不该反馈给终端里的程序，也不该让它中断）。
// 注意 owner / viewer 的门控不在这里做，而是由调用方决定「是否 load 这个 addon」——
// addon 的读应答走 terminal.input() → onData，双端都 load 会让 CLI 收到双份应答。
export function createTerminalClipboardProvider(
  writeText: (text: string) => Promise<unknown>,
): IClipboardProvider {
  return {
    readText: (): string => '',
    writeText: (_selection: string, text: string): void | Promise<void> => {
      if (!text) return;
      return writeText(text).then(
        () => undefined,
        () => undefined,
      );
    },
  };
}
