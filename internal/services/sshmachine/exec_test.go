package sshmachine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"xkpane/internal/storage"
)

// joined 把 argv 用 \x00 连接，便于断言「相邻两元素」这种顺序关系。
func joined(args []string) string { return strings.Join(args, "\x00") }

func TestBuildRemoteExecArgs_KeyWithPortAndIdentity(t *testing.T) {
	m := &storage.SshMachine{
		Host: "example.com", Port: 2222, Username: "deploy",
		AuthMethod: "key", IdentityFile: `C:\keys\id_ed25519`,
	}
	args, err := buildRemoteExecArgs(m)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	j := joined(args)
	if !strings.Contains(j, "-o\x00BatchMode=yes") {
		t.Errorf("missing BatchMode option: %v", args)
	}
	if !strings.Contains(j, "-o\x00StrictHostKeyChecking=accept-new") {
		t.Errorf("missing accept-new option: %v", args)
	}
	if !strings.Contains(j, "-p\x002222") {
		t.Errorf("expected -p 2222: %v", args)
	}
	if !strings.Contains(j, "-i\x00"+`C:\keys\id_ed25519`) {
		t.Errorf("expected -i identity: %v", args)
	}
	// 末两元素必须是 "--" 与 user@host（host 不被当选项 + target 正确）。
	if n := len(args); n < 2 || args[n-2] != "--" || args[n-1] != "deploy@example.com" {
		t.Errorf("expected tail [--, deploy@example.com], got %v", args[max(0, len(args)-2):])
	}
}

func TestBuildRemoteExecArgs_AgentNoIdentityDefaultPort(t *testing.T) {
	m := &storage.SshMachine{Host: "h", Port: 22, Username: "u", AuthMethod: "agent"}
	args, err := buildRemoteExecArgs(m)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	j := joined(args)
	if strings.Contains(j, "-i\x00") {
		t.Errorf("agent auth must not pass -i: %v", args)
	}
	if strings.Contains(j, "-p\x00") {
		t.Errorf("port 22 must not pass -p: %v", args)
	}
	if args[len(args)-1] != "u@h" {
		t.Errorf("expected target u@h, got %v", args)
	}
}

func TestBuildUploadArgs_UsesUppercasePortAndRemoteSpec(t *testing.T) {
	m := &storage.SshMachine{Host: "h", Port: 2200, Username: "u", AuthMethod: "key", IdentityFile: "/k"}
	args, err := buildUploadArgs(m, `C:\tmp\a.txt`, "/remote/a.txt")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	j := joined(args)
	if !strings.Contains(j, "-P\x002200") { // scp 端口是大写 -P
		t.Errorf("expected uppercase -P 2200: %v", args)
	}
	if strings.Contains(j, "-p\x002200") {
		t.Errorf("scp must not use lowercase -p: %v", args)
	}
	// 末两元素：本地路径 + user@host:remotePath。
	n := len(args)
	if args[n-2] != `C:\tmp\a.txt` || args[n-1] != "u@h:/remote/a.txt" {
		t.Errorf("expected tail [local, u@h:/remote/a.txt], got %v", args[n-2:])
	}
}

func TestAuthArgs_RejectsPasswordMachine(t *testing.T) {
	m := &storage.SshMachine{Host: "h", Username: "u", AuthMethod: "password"}
	if _, err := authArgs(m); err == nil {
		t.Fatal("password machine must be rejected for non-interactive channel")
	}
}

func TestAuthArgs_RejectsInjectionInHostUser(t *testing.T) {
	for _, m := range []*storage.SshMachine{
		{Host: "h;rm -rf", Username: "u", AuthMethod: "agent"},
		{Host: "-oProxyCommand=evil", Username: "u", AuthMethod: "agent"},
		{Host: "h", Username: "u`whoami`", AuthMethod: "agent"},
	} {
		if _, err := authArgs(m); err == nil {
			t.Errorf("expected rejection for host=%q user=%q", m.Host, m.Username)
		}
	}
}

func TestUploadArgs_NotReachedWhenRemotePathDangerous(t *testing.T) {
	// buildUploadArgs 本身不校验 remotePath（由 Upload 前置 hasDangerousChars 拦），
	// 这里直接验证守卫函数对危险远端路径的判定，锁住注入面。
	for _, p := range []string{"/a;rm", "/a`x`", "/a|b", "/a$x", "/a\nb"} {
		if !hasDangerousChars(p) {
			t.Errorf("expected %q flagged dangerous", p)
		}
	}
	if hasDangerousChars("/home/u/dir name/a.txt") {
		t.Error("normal path with space must be allowed")
	}
}

func TestOpTimeout_DefaultAndCap(t *testing.T) {
	if got := opTimeout(0, defaultExecTimeout); got != defaultExecTimeout {
		t.Errorf("0 should fall back to default, got %v", got)
	}
	if got := opTimeout(-5, defaultUploadTimeout); got != defaultUploadTimeout {
		t.Errorf("negative should fall back to default, got %v", got)
	}
	if got := opTimeout(30, defaultExecTimeout); got != 30*time.Second {
		t.Errorf("explicit 30s expected, got %v", got)
	}
	if got := opTimeout(99999, defaultExecTimeout); got != maxOpTimeout {
		t.Errorf("over-cap should clamp to %v, got %v", maxOpTimeout, got)
	}
}

func TestExitCodeFrom(t *testing.T) {
	if code, err := exitCodeFrom(nil); code != 0 || err != nil {
		t.Errorf("nil → (0,nil), got (%d,%v)", code, err)
	}
	// 非 ExitError（如启动失败 / 超时被杀的通用错误）→ 返回 error 让上层报「错误: …」。
	sentinel := errors.New("boom")
	if code, err := exitCodeFrom(sentinel); code != -1 || !errors.Is(err, sentinel) {
		t.Errorf("generic err → (-1, err), got (%d,%v)", code, err)
	}
}
