package terminal

import (
	"strings"
	"testing"
)

// remoteString 取 argv 最后一段（远端命令字符串）。
func remoteString(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[len(args)-1]
}

func TestBuildSSHArgsKeyAuth(t *testing.T) {
	args := buildSSHArgs(SSHSpec{
		Host: "devbox.local", User: "dev", Port: 2222,
		IdentityFile: "/home/me/.ssh/id_ed25519", RemotePath: "/srv/app",
	}, "claude", nil)

	joined := strings.Join(args, " ")
	for _, want := range []string{"-tt", "-o ConnectTimeout=10", "-o ServerAliveInterval=15", "-p 2222", "-i /home/me/.ssh/id_ed25519", "dev@devbox.local"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q; got %v", want, args)
		}
	}
	if got := remoteString(args); got != "cd '/srv/app' && claude" {
		t.Errorf("remote = %q, want \"cd '/srv/app' && claude\"", got)
	}
}

func TestBuildSSHArgsDefaultPortNoFlag(t *testing.T) {
	args := buildSSHArgs(SSHSpec{Host: "h", User: "u", Port: 22, RemotePath: "~"}, "none", nil)
	if strings.Contains(strings.Join(args, " "), "-p ") {
		t.Errorf("port 22 should not emit -p; got %v", args)
	}
	// remotePath "~" 跳过 cd；none → 裸 shell。
	if got := remoteString(args); got != "exec $SHELL -l" {
		t.Errorf("remote = %q, want \"exec $SHELL -l\"", got)
	}
}

func TestBuildSSHArgsNoUserNoIdentity(t *testing.T) {
	args := buildSSHArgs(SSHSpec{Host: "h"}, "codex", nil)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "@") {
		t.Errorf("no user → target should be bare host; got %v", args)
	}
	if strings.Contains(joined, "-i ") {
		t.Errorf("no identityFile → no -i; got %v", args)
	}
	if got := remoteString(args); got != "codex --full-auto" {
		t.Errorf("remote = %q, want codex --full-auto", got)
	}
}

func TestBuildSSHArgsProviderEnvExportedOnlyForRealCLI(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "sk-abc'123", "bad key": "x"}

	// 真 CLI：导出合法 key（值单引号转义），跳过非法 key。
	remoteCLI := remoteString(buildSSHArgs(SSHSpec{Host: "h", RemotePath: "/w"}, "claude", env))
	if !strings.Contains(remoteCLI, `export ANTHROPIC_API_KEY='sk-abc'\''123'`) {
		t.Errorf("provider env not exported/escaped: %q", remoteCLI)
	}
	if strings.Contains(remoteCLI, "bad key") {
		t.Errorf("invalid env key leaked: %q", remoteCLI)
	}
	if !strings.HasSuffix(remoteCLI, "cd '/w' && claude") {
		t.Errorf("remote tail wrong: %q", remoteCLI)
	}

	// none（裸 shell）：不导出 provider env。
	remoteShell := remoteString(buildSSHArgs(SSHSpec{Host: "h"}, "none", env))
	if strings.Contains(remoteShell, "export") {
		t.Errorf("bare shell should not export provider env: %q", remoteShell)
	}
}

func TestShellEscape(t *testing.T) {
	cases := map[string]string{
		"plain":      "'plain'",
		"a b":        "'a b'",
		"x'y":        `'x'\''y'`,
		"$(rm -rf)":  "'$(rm -rf)'",
		"a;b|c&d":    "'a;b|c&d'",
	}
	for in, want := range cases {
		if got := shellEscape(in); got != want {
			t.Errorf("shellEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsValidEnvKey(t *testing.T) {
	valid := []string{"FOO", "_x", "A1_B2", "lower_ok"}
	invalid := []string{"", "1ABC", "A-B", "A B", "A.B", "$X"}
	for _, k := range valid {
		if !isValidEnvKey(k) {
			t.Errorf("isValidEnvKey(%q) = false, want true", k)
		}
	}
	for _, k := range invalid {
		if isValidEnvKey(k) {
			t.Errorf("isValidEnvKey(%q) = true, want false", k)
		}
	}
}
