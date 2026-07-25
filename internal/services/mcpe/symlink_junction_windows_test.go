//go:build windows

package mcpe

// 守门测试：behavior_packs 下 MCStudio 留下的目录联接（junction）必须被 removeSymlinksIn 清掉。
//
// 回归背景：junction 在 Go 里 Lstat 得到 ModeIrregular 而**不带** ModeSymlink，早先只判
// os.ModeSymlink 会漏掉它。残留 junction 与本程序按 uuid 新建的 symlink 同时指向同一包目录，
// 引擎便见到同一 pack_id 的两个入口，该包整包异常并连带桥模组 python 不加载 —— 表现就是
// MC PE 调试桥永远停在“等待中”。

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"xkpane/internal/utils/procx"
)

// mklinkJunction 用 cmd 内建 mklink /J 建目录联接（junction 不需要 symlink 特权，普通权限可跑）。
func mklinkJunction(t *testing.T, link, target string) {
	t.Helper()
	err := procx.Run(context.Background(), procx.Spec{
		Name: "cmd",
		Args: []string{"/c", "mklink", "/J", link, target},
	})
	if err != nil {
		t.Skipf("mklink /J 不可用，跳过 junction 测试: %v", err)
	}
}

func TestRemoveSymlinksInRemovesJunction(t *testing.T) {
	tmp := t.TempDir()
	// 被链接的“项目包”目录，内含一个文件用于验证数据未被误删。
	src := filepath.Join(tmp, "src_pack")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(src, "manifest.json")
	if err := os.WriteFile(payload, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// 模拟 behavior_packs 目录：一条 junction + 一个实体目录。
	packs := filepath.Join(tmp, "behavior_packs")
	if err := os.MkdirAll(packs, 0o755); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(packs, "behavior_pack_IXLiDy2K")
	mklinkJunction(t, junction, src)

	realDir := filepath.Join(packs, "183b0dc7-bridge")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 前置断言：junction 确实不带 ModeSymlink（否则本测试就失去意义）。
	fi, err := os.Lstat(junction)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("预期 junction 不带 ModeSymlink；本测试的前提已变化")
	}
	if !isLinkEntry(fi) {
		t.Fatal("isLinkEntry 未把 junction 认作链接")
	}

	if err := removeSymlinksIn(packs); err != nil {
		t.Fatalf("removeSymlinksIn: %v", err)
	}

	if _, err := os.Lstat(junction); !os.IsNotExist(err) {
		t.Errorf("junction 应被删除，Lstat err=%v", err)
	}
	// 只删链接、不动被链接的数据。
	if _, err := os.Stat(payload); err != nil {
		t.Errorf("被链接目录内的文件不应受影响: %v", err)
	}
	// 实体目录（桥模组解压目录就是实体目录）必须保留。
	if !isDir(realDir) {
		t.Error("实体子目录应保留")
	}
}
