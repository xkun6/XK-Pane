package workspace

import (
	"encoding/json"
	"testing"
)

func TestAddSSHProject(t *testing.T) {
	svc, ctx := newTestService(t)
	ws, err := svc.Create(ctx, "remote-ws", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	cfg := SshProjectConfig{
		MachineID: "m1", Host: "devbox.local", Port: 2222, User: "dev",
		RemotePath: "/srv/app", IdentityFile: "/home/me/.ssh/id_ed25519", AuthMethod: "key",
	}
	wp, err := svc.AddSSHProject(ctx, ws.ID, cfg)
	if err != nil {
		t.Fatalf("AddSSHProject: %v", err)
	}
	// 合成键填 Path（含 machineId + remotePath）。
	if wp.Path != "ssh://m1/srv/app" {
		t.Errorf("synthetic path = %q, want ssh://m1/srv/app", wp.Path)
	}
	// SSH 列是 cfg 的 JSON（端口默认补 22 已在传入值 2222）。
	if wp.SSH == nil {
		t.Fatal("SSH column nil")
	}
	var got SshProjectConfig
	if err := json.Unmarshal([]byte(*wp.SSH), &got); err != nil {
		t.Fatalf("SSH json: %v", err)
	}
	if got.Host != "devbox.local" || got.Port != 2222 || got.AuthMethod != "key" {
		t.Errorf("SSH config roundtrip wrong: %+v", got)
	}
}

func TestAddSSHProjectIdempotent(t *testing.T) {
	svc, ctx := newTestService(t)
	ws, _ := svc.Create(ctx, "remote-ws", nil)
	cfg := SshProjectConfig{MachineID: "m1", Host: "h", RemotePath: "~/x"}

	a, err := svc.AddSSHProject(ctx, ws.ID, cfg)
	if err != nil {
		t.Fatalf("add1: %v", err)
	}
	b, err := svc.AddSSHProject(ctx, ws.ID, cfg)
	if err != nil {
		t.Fatalf("add2: %v", err)
	}
	if a.ID != b.ID {
		t.Errorf("same machine+remotePath should dedup: %s vs %s", a.ID, b.ID)
	}
	list, _ := svc.List(ctx)
	if len(list) != 1 || len(list[0].Projects) != 1 {
		t.Errorf("expected 1 project after idempotent add, got %d", len(list[0].Projects))
	}
}

func TestAddSSHProjectDefaults(t *testing.T) {
	svc, ctx := newTestService(t)
	ws, _ := svc.Create(ctx, "remote-ws", nil)

	// 端口缺省 22；remotePath 空 → 合成键用 ~。
	wp, err := svc.AddSSHProject(ctx, ws.ID, SshProjectConfig{MachineID: "m2", Host: "h"})
	if err != nil {
		t.Fatalf("AddSSHProject: %v", err)
	}
	if wp.Path != "ssh://m2/~" {
		t.Errorf("synthetic path = %q, want ssh://m2/~", wp.Path)
	}
	var got SshProjectConfig
	_ = json.Unmarshal([]byte(*wp.SSH), &got)
	if got.Port != 22 {
		t.Errorf("port default = %d, want 22", got.Port)
	}

	// host 为空 → 报错。
	if _, err := svc.AddSSHProject(ctx, ws.ID, SshProjectConfig{Host: "  "}); err == nil {
		t.Error("empty host should error")
	}
}
