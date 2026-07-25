// xterm 隐藏 textarea 的 IME 缓冲复位 —— 治「偶尔打中文重复一段文字」。
//
// 病理（xterm.js 6.0.0）：IME 上屏的字会留在隐藏 textarea 里，全库只有三处会动它 —— 按
// Enter / Ctrl+C（CoreBrowserTerminal 的 result.key === CR/ETX 分支）、走 xterm 自己的粘贴、
// 右键把选区塞进去 —— 于是一行中文会一直累积。而 CompositionHelper 是按**下标**从这个缓冲里
// substring 出要发送的内容（compositionstart 记 start = value.length；compositionend 的
// setTimeout(0) 里发 value.substring(start) 一路切到末尾）。下标与实际内容一旦错开，就会把已经
// 发过的旧内容整段重发 —— 屏幕上就是重复一段中文。错开的已知路径：
//   - IME 替换候选导致「长度不变、内容变」→ CompositionHelper 直接把整个 textarea 值当输入发；
//   - 回删 / 重新编辑已上屏内容 → 它用 String.replace 求 diff 失配，diff 退化成全文；
//   - 打字过快 → compositionupdate 里延后一个宏任务才更新的 end 下标滞后，切出过期区间。
// 都是时序 / 交互巧合，所以表现为「偶尔」，且重复长度 = 缓冲里累积的那段（本行未回车的中文）。
//
// 对策：每次 compositionend 之后的下一个宏任务把缓冲清空，让它永不累积（重复量从「整行」压到
// 「最多一个词」）。不改 xterm 源码、不碰它的私有状态。
//
// 时序铁律（改坏直接丢字，勿动）：
//   ① compositionend 必须注册在**冒泡**阶段。xterm 的 compositionend listener 同为冒泡且注册更早
//      （term.open 内部），于是它先跑、先排自己那个 setTimeout(0)（真正的发送），我们后跑、后排 →
//      清空必然发生在发送之后。改成 capture 就会抢在发送前清空，substring 取空 → 中文直接丢。
//   ② 新一轮组合已开始时跳过清空：新组合的起点下标是按「清空前的长度」记的，此刻清空会让它取空。
//   ③ compositionend 之后又来了 keydown 则跳过清空：IME 活跃时的 keydown(229) 会让 xterm 走
//      _handleAnyTextareaChanges —— 它先拍 oldValue 快照、下一个宏任务再比长度，我们的清空插在中间
//      会让它误判「变短了」而补发一个 DEL 把字符删掉。

export interface ImeBufferResetOptions {
  /** 排一个「下一个宏任务」（默认 setTimeout 0）。测试注入以精确控制时序。 */
  schedule?: (task: () => void) => void;
}

// 给 xterm 的隐藏 textarea 挂上 IME 缓冲复位；返回解绑函数。
export function attachImeBufferReset(
  textarea: HTMLTextAreaElement,
  options: ImeBufferResetOptions = {},
): () => void {
  const schedule =
    options.schedule ??
    ((task: () => void) => {
      setTimeout(task, 0);
    });

  let composing = false;
  // 按键序号：compositionend 之后再来任何 keydown 就放弃本次清空（铁律 ③）。
  let keydownSeq = 0;

  const onCompositionStart = (): void => {
    composing = true;
  };

  const onCompositionEnd = (): void => {
    composing = false;
    const seqAtEnd = keydownSeq;
    schedule(() => {
      if (composing || keydownSeq !== seqAtEnd) return;
      if (textarea.value) textarea.value = '';
    });
  };

  // keydown 只计数，故用 capture（更早、且不受 xterm 在 cancel() 里 stopPropagation 的影响）；
  // composition 两件套必须留在冒泡阶段（铁律 ①）。
  const onKeyDown = (): void => {
    keydownSeq += 1;
  };

  textarea.addEventListener('compositionstart', onCompositionStart);
  textarea.addEventListener('compositionend', onCompositionEnd);
  textarea.addEventListener('keydown', onKeyDown, true);

  return () => {
    textarea.removeEventListener('compositionstart', onCompositionStart);
    textarea.removeEventListener('compositionend', onCompositionEnd);
    textarea.removeEventListener('keydown', onKeyDown, true);
  };
}
