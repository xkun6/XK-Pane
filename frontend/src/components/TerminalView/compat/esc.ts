// ESC / 字符串终止符常量。用 String.fromCharCode 显式构造，避免源码里写 \x1b/
// 在不同工具链下的转义歧义。ESC=0x1B；ST(OSC String Terminator)=ESC + '\'。
export const ESC = String.fromCharCode(27);
export const ST = ESC + '\\';
