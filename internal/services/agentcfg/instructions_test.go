package agentcfg

import (
	"os"
	"path/filepath"
	"testing"
)

// ---- 1. 文件不存在 → 读到空串（不是错误） ----

func TestReadInstructionFileMissingReturnsEmpty(t *testing.T) {
	svc := newTestService(t)

	got, err := svc.ReadInstructionFile(appTypeClaude)
	if err != nil {
		t.Fatalf("ReadInstructionFile(claude): %v", err)
	}
	if got != "" {
		t.Fatalf("got = %q, want empty", got)
	}

	got, err = svc.ReadInstructionFile(appTypeCodex)
	if err != nil {
		t.Fatalf("ReadInstructionFile(codex): %v", err)
	}
	if got != "" {
		t.Fatalf("got = %q, want empty", got)
	}
}

// ---- 2. 写后读回一致 + 写会创建父目录 ----

func TestWriteThenReadInstructionFileRoundTrip(t *testing.T) {
	svc := newTestService(t)

	claudeDir := filepath.Join(svc.home, ".claude")
	if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
		t.Fatalf("前置条件不满足：%s 不应预先存在", claudeDir)
	}

	const content = "# CLAUDE.md\n\n一些全局指令\n"
	if err := svc.WriteInstructionFile(appTypeClaude, content); err != nil {
		t.Fatalf("WriteInstructionFile(claude): %v", err)
	}
	if _, err := os.Stat(claudeDir); err != nil {
		t.Fatalf("写入未创建父目录 %s: %v", claudeDir, err)
	}

	got, err := svc.ReadInstructionFile(appTypeClaude)
	if err != nil {
		t.Fatalf("ReadInstructionFile(claude): %v", err)
	}
	if got != content {
		t.Fatalf("got = %q, want %q", got, content)
	}

	// codex 侧同理，路径不同（AGENTS.md 而非 CLAUDE.md）。
	const codexContent = "# AGENTS.md\n\ncodex 专属指令\n"
	if err := svc.WriteInstructionFile(appTypeCodex, codexContent); err != nil {
		t.Fatalf("WriteInstructionFile(codex): %v", err)
	}
	got, err = svc.ReadInstructionFile(appTypeCodex)
	if err != nil {
		t.Fatalf("ReadInstructionFile(codex): %v", err)
	}
	if got != codexContent {
		t.Fatalf("got = %q, want %q", got, codexContent)
	}
	if svc.instructionFilePath(appTypeClaude) == svc.instructionFilePath(appTypeCodex) {
		t.Fatal("claude / codex 指令文件路径不应相同")
	}
}

// ---- 3. backupLiveOnce 语义：只在首次写前备份一次，此后不再覆盖 .xkpane-bak ----

func TestWriteInstructionFileBacksUpOnlyOnce(t *testing.T) {
	svc := newTestService(t)
	path := svc.instructionFilePath(appTypeClaude)
	bakPath := path + ".xkpane-bak"

	const original = "用户手工维护的原始内容\n"
	writeTextFile(t, path, original)

	if err := svc.WriteInstructionFile(appTypeClaude, "第一次服务写入\n"); err != nil {
		t.Fatalf("WriteInstructionFile(第一次): %v", err)
	}
	bak, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("读取 %s: %v", bakPath, err)
	}
	if string(bak) != original {
		t.Fatalf("首次写前备份内容不对: got %q, want %q", string(bak), original)
	}

	if err := svc.WriteInstructionFile(appTypeClaude, "第二次服务写入\n"); err != nil {
		t.Fatalf("WriteInstructionFile(第二次): %v", err)
	}
	bak, err = os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("读取 %s: %v", bakPath, err)
	}
	if string(bak) != original {
		t.Fatalf("第二次写入后备份被覆盖: got %q, want %q（应仍是第一次写前的内容）", string(bak), original)
	}

	got, err := svc.ReadInstructionFile(appTypeClaude)
	if err != nil {
		t.Fatalf("ReadInstructionFile: %v", err)
	}
	if got != "第二次服务写入\n" {
		t.Fatalf("live 内容 got = %q, want 第二次服务写入", got)
	}
}

// ---- 4. 非法 appType 报错 ----

func TestInstructionFileRejectsIllegalAppType(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.ReadInstructionFile("gemini"); err == nil {
		t.Fatal("ReadInstructionFile(非法 appType) 应报错")
	}
	if err := svc.WriteInstructionFile("gemini", "x"); err == nil {
		t.Fatal("WriteInstructionFile(非法 appType) 应报错")
	}
}
