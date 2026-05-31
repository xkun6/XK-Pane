package localhistory

import (
	"context"
	"testing"
)

func TestPathMatchesIgnore(t *testing.T) {
	patterns := DefaultHistoryConfig().IgnorePatterns // node_modules/** .git/** target/** dist/** build/** *.log *.lock .xkpanes/**
	cases := []struct {
		path string
		want bool
	}{
		// 嵌套依赖目录（回归：早期 dir/** 根前缀漏匹配 frontend/node_modules）
		{"frontend/node_modules/.bin/vite.CMD", true},
		{"frontend/node_modules/.pnpm/@vitest+mocker@4.1.7/node_modules/.modules.yaml", true},
		{"a/b/.git/HEAD", true},
		{"frontend/dist/assets/x.js", true},
		{"pkg/target/debug/app", true},
		// 后缀
		{"logs/app.log", true},
		{"pnpm-lock.lock", true},
		// 普通源码不忽略
		{"internal/app/app.go", false},
		{"frontend/src/App.tsx", false},
		// 不误匹配相邻同前缀
		{"target2/foo", false},
		{"frontend/my-node_modules/x", false},
	}
	for _, c := range cases {
		if got := PathMatchesIgnore(c.path, patterns); got != c.want {
			t.Errorf("PathMatchesIgnore(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestCleanupPurgesIgnoredVersions(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	// 一个正常源码文件 + 一个嵌套 node_modules 文件（不受默认 ignore 根前缀漏匹配影响——现已任意深度匹配）。
	if _, err := s.SaveVersion(ctx, "internal/app/app.go", []byte("package app\n"), false, "main", 0); err != nil {
		t.Fatalf("save normal: %v", err)
	}
	if _, err := s.SaveVersion(ctx, "frontend/node_modules/.bin/vite.CMD", []byte("@echo off\n"), false, "main", 0); err != nil {
		t.Fatalf("save junk: %v", err)
	}

	if err := s.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	// node_modules 版本应被清掉，正常文件保留。
	junk, err := s.ListVersions(ctx, "frontend/node_modules/.bin/vite.CMD")
	if err != nil {
		t.Fatalf("list junk: %v", err)
	}
	if len(junk) != 0 {
		t.Errorf("ignored versions not purged: %d remain", len(junk))
	}
	normal, err := s.ListVersions(ctx, "internal/app/app.go")
	if err != nil {
		t.Fatalf("list normal: %v", err)
	}
	if len(normal) != 1 {
		t.Errorf("normal version should survive cleanup: got %d", len(normal))
	}
}
