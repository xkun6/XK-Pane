package sshmachine

import (
	"context"
	"path/filepath"
	"testing"

	"xkpane/internal/storage"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(storage.NewHolder(db))
}

func passwordMachine(name string) MachineInput {
	return MachineInput{
		Name: name, Host: "devbox.local", Port: 22, User: "dev",
		AuthMethod: "password", Password: "s3cret", DefaultPath: "~/projects",
		Tags: []string{"prod"},
	}
}

func TestAddEncryptsPasswordAndMasksDTO(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	dto, err := svc.Add(ctx, passwordMachine("box1"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !dto.HasStoredPassword {
		t.Error("DTO.HasStoredPassword = false, want true")
	}
	if len(dto.Tags) != 1 || dto.Tags[0] != "prod" {
		t.Errorf("DTO.Tags = %v, want [prod]", dto.Tags)
	}

	// 落库密文：裸读 password 列既非明文也非空。
	var raw storage.SshMachine
	if err := svc.db().First(&raw, "id = ?", dto.ID).Error; err != nil {
		t.Fatalf("raw read: %v", err)
	}
	if raw.Password == "" || raw.Password == "s3cret" {
		t.Fatalf("password not encrypted at rest: %q", raw.Password)
	}

	// 非绑定出口解密回明文。
	m, err := ResolveForSSH(ctx, svc.db(), dto.ID)
	if err != nil || m == nil {
		t.Fatalf("ResolveForSSH: %v / %v", m, err)
	}
	if m.Password != "s3cret" {
		t.Errorf("decrypt roundtrip = %q, want s3cret", m.Password)
	}
}

func TestUpdateEmptyPasswordPreserves(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	dto, _ := svc.Add(ctx, passwordMachine("box1"))

	in := passwordMachine("box1")
	in.Password = "" // 空 = 保留原密码
	in.Description = "edited"
	if err := svc.Update(ctx, dto.ID, in); err != nil {
		t.Fatalf("Update: %v", err)
	}
	m, _ := ResolveForSSH(ctx, svc.db(), dto.ID)
	if m == nil || m.Password != "s3cret" {
		t.Errorf("password not preserved on empty update: %v", m)
	}
	if m.Description != "edited" {
		t.Errorf("description not patched: %q", m.Description)
	}
}

func TestUpdateClearStoredPassword(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	dto, _ := svc.Add(ctx, passwordMachine("box1"))

	in := passwordMachine("box1")
	in.ClearStoredPassword = true
	if err := svc.Update(ctx, dto.ID, in); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := svc.Get(ctx, dto.ID)
	if got.HasStoredPassword {
		t.Error("password not cleared")
	}
}

func TestKeyAuthDropsPassword(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	in := passwordMachine("box1")
	in.AuthMethod = "key" // key 认证不存密码，即使带了 password 输入
	in.IdentityFile = "/home/dev/.ssh/id_ed25519"
	dto, err := svc.Add(ctx, in)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if dto.HasStoredPassword {
		t.Error("key-auth machine stored a password")
	}
}

func TestNameUniquenessCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	if _, err := svc.Add(ctx, passwordMachine("Box")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := svc.Add(ctx, passwordMachine("box")); err == nil {
		t.Error("duplicate name (case-insensitive) was allowed")
	}
}

func TestValidationRejectsBadInput(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	cases := map[string]MachineInput{
		"empty name":      {Name: "", Host: "h"},
		"empty host":      {Name: "n", Host: ""},
		"host injection":  {Name: "n", Host: "h;rm -rf"},
		"bad port":        {Name: "n", Host: "h", Port: 70000},
		"relative path":   {Name: "n", Host: "h", DefaultPath: "relative/dir"},
		"identity dotdot": {Name: "n", Host: "h", IdentityFile: "../../etc/passwd"},
	}
	for name, in := range cases {
		if _, err := svc.Add(ctx, in); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}

func TestRemove(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	dto, _ := svc.Add(ctx, passwordMachine("box1"))
	if err := svc.Remove(ctx, dto.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := svc.Remove(ctx, dto.ID); err == nil {
		t.Error("removing missing machine should error")
	}
	list, _ := svc.List(ctx)
	if len(list) != 0 {
		t.Errorf("list after remove = %d, want 0", len(list))
	}
}
