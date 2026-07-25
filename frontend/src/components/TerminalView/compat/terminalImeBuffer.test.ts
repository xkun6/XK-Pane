// @vitest-environment jsdom
//
// 该文件需真实 DOM（textarea + composition/keyboard 事件派发），故 per-file 切环境
// （同 src/terminal/layoutScheduler.test.ts 的做法）；仓库默认 test 环境是 node。
import { beforeEach, describe, expect, it } from 'vitest';
import { attachImeBufferReset } from './terminalImeBuffer';

// 用手动 schedule 队列替代 setTimeout，精确复现「xterm 先发送、我们后清空」的宏任务次序。
function setup() {
  const textarea = document.createElement('textarea');
  document.body.appendChild(textarea);
  const queue: Array<() => void> = [];
  const detach = attachImeBufferReset(textarea, { schedule: (task) => queue.push(task) });
  return {
    textarea,
    detach,
    /** 跑完排队的宏任务（等价于 xterm 那批 setTimeout(0) 之后轮到我们）。 */
    flush: () => {
      const tasks = queue.splice(0, queue.length);
      for (const task of tasks) task();
    },
    pendingCount: () => queue.length,
    compositionStart: () => textarea.dispatchEvent(new CompositionEvent('compositionstart')),
    compositionEnd: () => textarea.dispatchEvent(new CompositionEvent('compositionend')),
    keyDown: () => textarea.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true })),
  };
}

describe('attachImeBufferReset', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  it('一轮中文上屏后清空累积缓冲', () => {
    const t = setup();
    t.compositionStart();
    t.textarea.value = '你好';
    t.compositionEnd();

    // 清空必须是排队的、不是同步的 —— 同步就会抢在 xterm 读 substring 之前，中文会丢。
    expect(t.textarea.value).toBe('你好');
    t.flush();
    expect(t.textarea.value).toBe('');
  });

  it('累积多轮后仍只清一次，缓冲不增长', () => {
    const t = setup();
    for (const word of ['你好', '你好世界', '你好世界啊']) {
      t.compositionStart();
      t.textarea.value = word;
      t.compositionEnd();
      t.flush();
      expect(t.textarea.value).toBe('');
    }
  });

  it('新一轮组合已开始时跳过清空（否则新组合的起点下标会取空 → 丢字）', () => {
    const t = setup();
    t.compositionStart();
    t.textarea.value = '你好';
    t.compositionEnd();
    // 连续快打：清空任务还没跑，下一轮组合已经开始。
    t.compositionStart();
    t.flush();
    expect(t.textarea.value).toBe('你好');
  });

  it('compositionend 之后又来 keydown 时跳过清空（防 xterm 误补 DEL 删字）', () => {
    const t = setup();
    t.compositionStart();
    t.textarea.value = '你好';
    t.compositionEnd();
    t.keyDown();
    t.flush();
    expect(t.textarea.value).toBe('你好');
  });

  it('组合期间的 keydown 不影响本轮清空', () => {
    const t = setup();
    t.compositionStart();
    t.keyDown(); // IME 组合中的按键（keyCode 229）发生在 compositionend 之前
    t.textarea.value = '你好';
    t.compositionEnd();
    t.flush();
    expect(t.textarea.value).toBe('');
  });

  it('解绑后不再响应事件', () => {
    const t = setup();
    t.detach();
    t.compositionStart();
    t.textarea.value = '你好';
    t.compositionEnd();
    expect(t.pendingCount()).toBe(0);
    t.flush();
    expect(t.textarea.value).toBe('你好');
  });
});
