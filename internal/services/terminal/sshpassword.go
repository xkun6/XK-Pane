package terminal

import (
	"context"
	"strings"
)

// SSH 密码自动应答 —— 忠实移植 cc-pane terminal_service 的 SshAuthRuntime / looks_like_ssh_password_prompt。
//
// 仅 RuntimeKind=="ssh" 且 AuthMethod=="password" 且机器存有密码时武装。本地 ssh 客户端在 PTY 里
// 打出 "<user>@<host>'s password:" 时，readLoop 检出该 prompt 并一次性把解密后的明文密码写回 PTY。
// 一次性（autoResponseSent 置位后永不再应答）：避免把登录密码误注入登录后远端出现的 "password:" 行
// （远端 sudo / 业务程序提示）。明文密码仅活在本会话内存，**绝不经 bindings 回前端**。

// SSHPasswordResolver 按 machineID 返回该机器存储的明文 SSH 登录密码。
// 返回 ("", nil) = 无密码 / 未知机器（→ 不武装，回退手动输入）；err = 解密/查库失败（同样回退）。
// 由 app 装配阶段经 SetSSHPasswordResolver 注入（闭包调 sshmachine.ResolveForSSH 解密）。
// ⚠️ 返回明文密码 —— 绝不暴露给前端，仅在 readLoop 内写进 PTY 应答密码 prompt。
type SSHPasswordResolver func(ctx context.Context, machineID string) (string, error)

// sshPromptBufferCap 是密码 prompt 滚动缓冲的尾部字节上限（对齐 cc-pane 512）。
const sshPromptBufferCap = 512

// sshAuthRuntime 是一次 SSH 会话的密码自动应答运行时。仅 readLoop 单 goroutine 读写 → 无需同步。
type sshAuthRuntime struct {
	promptBuffer     string // 最近输出的归一文本尾巴（跨 chunk 拼 prompt），封顶 sshPromptBufferCap
	savedPassword    string // 解密后的明文登录密码
	autoResponseSent bool   // 一次性：已自动应答则永不再触发
}

// normalizePromptText 归一一段 PTY 输出用于 prompt 匹配：换行归一（\r\n、\r → \n）+ 剥 ANSI。
// 对齐 cc-pane normalize_prompt_text。
func normalizePromptText(data string) string {
	return stripANSI(strings.ReplaceAll(strings.ReplaceAll(data, "\r\n", "\n"), "\r", "\n"))
}

// looksLikeSSHPasswordPrompt 判断一行是否为 SSH 登录密码 prompt（对齐 cc-pane looks_like_ssh_password_prompt）：
// 小写后不含 "passphrase"（排除加密私钥口令 prompt）且以 "password:" / "password: " 结尾。
func looksLikeSSHPasswordPrompt(line string) bool {
	lower := strings.ToLower(line)
	return !strings.Contains(lower, "passphrase") &&
		(strings.HasSuffix(lower, "password:") || strings.HasSuffix(lower, "password: "))
}

// passwordPromptInBuffer 取缓冲最后一行（最后一个 \n 之后、去尾部空白）判断是否密码 prompt。
// 纯函数，便于单测；prompt 通常单行抵达，但跨 chunk 拼接时也只看最后一行。
func passwordPromptInBuffer(buffer string) bool {
	lastLine := buffer
	if idx := strings.LastIndexByte(lastLine, '\n'); idx >= 0 {
		lastLine = lastLine[idx+1:]
	}
	lastLine = strings.TrimRight(lastLine, " \t\r\n")
	return looksLikeSSHPasswordPrompt(lastLine)
}

// armSSHPassword 据启动 spec 决定是否武装密码自动应答：仅 ssh + AuthMethod=="password" + resolver 已注入
// + 机器存有非空密码时返回运行时；其余（含解密失败）返回 nil（回退手动输入）。
func (s *Service) armSSHPassword(ctx context.Context, spec CreateSpec) *sshAuthRuntime {
	if spec.RuntimeKind != "ssh" || spec.SSH == nil || spec.SSH.AuthMethod != "password" {
		return nil
	}
	resolve := s.resolveSSHPassword
	if resolve == nil {
		return nil
	}
	pw, err := resolve(ctx, spec.SSH.MachineID)
	if err != nil {
		s.logger.Warn("load stored SSH password failed; manual prompt", "host", spec.SSH.Host, "err", err)
		return nil
	}
	if pw == "" {
		return nil
	}
	return &sshAuthRuntime{savedPassword: pw}
}

// feedSSHPassword 把一段输出喂给密码自动应答状态机：累积归一文本（尾部封顶），取最后一行匹配密码
// prompt → 一次性把明文密码 + "\n" 写进 PTY。sess.sshAuth==nil（本地/key/agent）即 no-op。仅 readLoop 调用。
func (s *Service) feedSSHPassword(sess *session, chunk string) {
	rt := sess.sshAuth
	if rt == nil || rt.autoResponseSent {
		return
	}
	rt.promptBuffer += normalizePromptText(chunk)
	if len(rt.promptBuffer) > sshPromptBufferCap {
		rt.promptBuffer = rt.promptBuffer[len(rt.promptBuffer)-sshPromptBufferCap:]
	}
	if !passwordPromptInBuffer(rt.promptBuffer) {
		return
	}
	if _, err := sess.pty.Write([]byte(rt.savedPassword + "\n")); err != nil {
		s.logger.Warn("ssh password auto-response write failed", "session", sess.id, "err", err)
		return
	}
	rt.autoResponseSent = true
	rt.promptBuffer = ""
	s.logger.Info("ssh password auto-responded", "session", sess.id)
}
