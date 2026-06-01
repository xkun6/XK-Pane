package mcpe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveSymlinksInNonexistentDir(t *testing.T) {
	if err := removeSymlinksIn(filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Fatalf("missing dir should be ok, got %v", err)
	}
}

func TestRemoveSymlinksPreservesRealSubdirs(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := removeSymlinksIn(tmp); err != nil {
		t.Fatal(err)
	}
	if !isDir(real) {
		t.Fatal("实体子目录应保留")
	}
}

// buildSymlinkBat 校验 + 生成 mklink /D 内容（不真弹 UAC，跨平台可测）。
func TestBuildSymlinkBatContent(t *testing.T) {
	content, err := buildSymlinkBat([]symlinkPair{
		{target: `C:\proj\bp`, link: `C:\game\behavior_packs\uuid-1`},
		{target: `C:\proj\rp`, link: `C:\game\resource_packs\uuid-2`},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.HasPrefix(content, "@echo off\r\n") {
		t.Errorf("missing @echo off header: %q", content)
	}
	if !strings.Contains(content, `mklink /D "C:\game\behavior_packs\uuid-1" "C:\proj\bp"`+"\r\n") {
		t.Errorf("missing/incorrect mklink line: %q", content)
	}
	if !strings.Contains(content, `mklink /D "C:\game\resource_packs\uuid-2" "C:\proj\rp"`+"\r\n") {
		t.Errorf("missing/incorrect second mklink line: %q", content)
	}
}

func TestBuildSymlinkBatRejectsInjection(t *testing.T) {
	_, err := buildSymlinkBat([]symlinkPair{
		{target: `C:\proj"&calc&"`, link: `C:\game\x`},
	})
	if err == nil {
		t.Fatal("含命令注入字符的路径应被拒绝")
	}
}

func TestBuildSymlinkBatEmptyIsEmptyHeader(t *testing.T) {
	content, err := buildSymlinkBat(nil)
	if err != nil {
		t.Fatal(err)
	}
	if content != "@echo off\r\n" {
		t.Errorf("empty pairs should yield just header, got %q", content)
	}
}
