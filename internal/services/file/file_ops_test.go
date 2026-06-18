package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 这些操作不触碰 s.holder/db，可用空 Service 直测；Trash 需系统回收站，留真机手测。

func TestValidName(t *testing.T) {
	ok := []string{"a.txt", "中文名.md", strings.Repeat("x", 255)}
	for _, n := range ok {
		if err := validName(n); err != nil {
			t.Errorf("validName(%q) 期望通过，得 %v", n, err)
		}
	}
	bad := []string{"", ".", "..", "a/b", `a\b`, "a\x00b", strings.Repeat("x", 256)}
	for _, n := range bad {
		if err := validName(n); err == nil {
			t.Errorf("validName(%q) 期望报错，却通过", n)
		}
	}
}

func TestCreateFileAndDir(t *testing.T) {
	s := &Service{}
	root := t.TempDir()
	ctx := context.Background()

	fp, err := s.CreateFile(ctx, root, "a.txt")
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if fi, err := os.Stat(fp); err != nil || fi.IsDir() {
		t.Fatalf("新文件不存在或是目录: %v", err)
	}
	// O_EXCL：重名报错，不覆盖
	if _, err := s.CreateFile(ctx, root, "a.txt"); err == nil {
		t.Error("CreateFile 重名期望报错")
	}

	dp, err := s.CreateDir(ctx, root, "sub")
	if err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if fi, err := os.Stat(dp); err != nil || !fi.IsDir() {
		t.Fatalf("新目录不存在或非目录: %v", err)
	}
	if _, err := s.CreateDir(ctx, root, "sub"); err == nil {
		t.Error("CreateDir 重名期望报错")
	}
	// 非法名拦截
	if _, err := s.CreateFile(ctx, root, "../escape"); err == nil {
		t.Error("CreateFile 非法名期望报错")
	}
}

func TestRename(t *testing.T) {
	s := &Service{}
	root := t.TempDir()
	ctx := context.Background()

	src := filepath.Join(root, "old.txt")
	if err := os.WriteFile(src, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	np, err := s.Rename(ctx, src, "new.txt")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if filepath.Base(np) != "new.txt" {
		t.Errorf("新名不符: %s", np)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("旧路径应不存在")
	}

	// 冲突：目标已存在
	other := filepath.Join(root, "taken.txt")
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rename(ctx, np, "taken.txt"); err == nil {
		t.Error("Rename 到已存在名期望报错")
	}
}

func TestMoveAndDuplicate(t *testing.T) {
	s := &Service{}
	root := t.TempDir()
	ctx := context.Background()

	src := filepath.Join(root, "f.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "dir")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	moved, err := s.Move(ctx, src, dst)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if filepath.Dir(moved) != dst {
		t.Errorf("移动目标目录不符: %s", moved)
	}

	// 再放一个同名文件，移动应加序号
	if err := os.WriteFile(src, []byte("data2"), 0o644); err != nil {
		t.Fatal(err)
	}
	moved2, err := s.Move(ctx, src, dst)
	if err != nil {
		t.Fatalf("Move2: %v", err)
	}
	if moved2 == moved || !strings.Contains(filepath.Base(moved2), "-2") {
		t.Errorf("同名移动应加 -2 序号，得 %s", moved2)
	}

	// Duplicate 在同目录生成副本
	dup, err := s.Duplicate(ctx, moved)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if dup == moved {
		t.Error("副本路径不应等于原路径")
	}
	if _, err := os.Stat(dup); err != nil {
		t.Errorf("副本不存在: %v", err)
	}
}

func TestSearchContent(t *testing.T) {
	s := &Service{}
	root := t.TempDir()
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello world\nfoo BAR baz"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.go"), []byte("package x\n// bar here"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 忽略目录里的命中不应出现
	nm := filepath.Join(root, "node_modules")
	if err := os.Mkdir(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nm, "c.txt"), []byte("bar in node_modules"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 二进制文件应被跳过
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0x00, 'b', 'a', 'r'}, 0o644); err != nil {
		t.Fatal(err)
	}

	// 不区分大小写匹配 "bar"：命中 a.txt(BAR) 与 sub/b.go(bar)，不含 node_modules 与二进制。
	res, err := s.SearchContent(ctx, root, "bar")
	if err != nil {
		t.Fatalf("SearchContent: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("命中数 = %d，期望 2：%+v", len(res), res)
	}
	for _, m := range res {
		if strings.Contains(m.Path, "node_modules") || strings.Contains(m.Path, "bin.dat") {
			t.Errorf("不应命中忽略/二进制：%s", m.Path)
		}
		if m.Line < 1 {
			t.Errorf("行号应 ≥1：%+v", m)
		}
	}

	// 空 query 返回空
	if r, _ := s.SearchContent(ctx, root, "   "); len(r) != 0 {
		t.Errorf("空 query 期望 0 命中，得 %d", len(r))
	}
}

func TestReadFileBase64(t *testing.T) {
	s := &Service{}
	root := t.TempDir()
	ctx := context.Background()

	p := filepath.Join(root, "a.png")
	if err := os.WriteFile(p, []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}
	uri, err := s.ReadFileBase64(ctx, p)
	if err != nil {
		t.Fatalf("ReadFileBase64: %v", err)
	}
	if !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Errorf("data URI 前缀不符: %.40s", uri)
	}
}
