package terminal

import (
	"context"
	"testing"
)

func TestLooksLikeSSHPasswordPrompt(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"alice@example.com's password:", true},
		{"alice@example.com's password: ", true},
		{"Password:", true},
		{"password: ", true},
		{"ROOT@HOST'S PASSWORD:", true}, // 大小写不敏感
		{"Enter passphrase for key '/home/u/.ssh/id_ed25519':", false}, // passphrase 排除
		{"the password: is hidden somewhere mid-line", false},          // 非结尾
		{"Last login: Tue", false},
		{"", false},
	}
	for _, c := range cases {
		if got := looksLikeSSHPasswordPrompt(c.line); got != c.want {
			t.Errorf("looksLikeSSHPasswordPrompt(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestNormalizePromptTextStripsANSIAndCR(t *testing.T) {
	// 含颜色 ANSI + \r\n + 裸 \r，归一后应得纯文本，prompt 在最后一行可被识别。
	in := "\x1b[2K\rfoo\r\n\x1b[31malice@host's password:\x1b[0m "
	got := normalizePromptText(in)
	if !passwordPromptInBuffer(got) {
		t.Fatalf("normalized buffer should match password prompt, got %q", got)
	}
}

func TestPasswordPromptInBufferLastLineOnly(t *testing.T) {
	// 仅看最后一行：早先出现 password 中段行不触发，真正的尾行 prompt 才触发。
	if passwordPromptInBuffer("type your password here\nWelcome to host\n$ ") {
		t.Error("non-prompt last line should not match")
	}
	if !passwordPromptInBuffer("Permission denied, please try again.\nalice@host's password: ") {
		t.Error("trailing password prompt after a denial line should match")
	}
}

func TestFeedSSHPasswordNotArmedNoPanic(t *testing.T) {
	// sshAuth==nil（本地/key/agent 会话）：feed 必须是安全 no-op（不解引用 nil pty）。
	svc := New(nil)
	sess := &session{id: "s1"}
	svc.feedSSHPassword(sess, "alice@host's password: ") // 不应 panic / 不应写 pty
}

func TestArmSSHPasswordGating(t *testing.T) {
	svc := New(nil)
	svc.SetSSHPasswordResolver(func(_ context.Context, machineID string) (string, error) {
		if machineID == "m-ok" {
			return "secret", nil
		}
		return "", nil
	})
	ctx := context.Background()

	// 非 ssh：不武装。
	if rt := svc.armSSHPassword(ctx, CreateSpec{RuntimeKind: "local"}); rt != nil {
		t.Error("local runtime should not arm")
	}
	// ssh 但 key 登录：不武装。
	if rt := svc.armSSHPassword(ctx, CreateSpec{RuntimeKind: "ssh", SSH: &SSHSpec{AuthMethod: "key", MachineID: "m-ok"}}); rt != nil {
		t.Error("key auth should not arm")
	}
	// ssh + password + 有密码：武装且缓存明文。
	rt := svc.armSSHPassword(ctx, CreateSpec{RuntimeKind: "ssh", SSH: &SSHSpec{AuthMethod: "password", MachineID: "m-ok"}})
	if rt == nil || rt.savedPassword != "secret" {
		t.Fatalf("password auth with stored secret should arm; got %+v", rt)
	}
	// ssh + password 但机器无密码：不武装。
	if rt := svc.armSSHPassword(ctx, CreateSpec{RuntimeKind: "ssh", SSH: &SSHSpec{AuthMethod: "password", MachineID: "m-none"}}); rt != nil {
		t.Error("password auth without stored secret should not arm")
	}
}
