package appinfo

import "testing"

// TestAppDirNameMatchesBuildMode 守门测试：保证 dev/release 数据目录隔离不变式。
// 无 build tag → dev → "XK-Pane-Dev"；`-tags production` → release → "XK-Pane"。
// 两种模式下都能跑（断言随 IsDev 分支），防止有人误改 devSuffix 造成 dev 写进 release 目录。
func TestAppDirNameMatchesBuildMode(t *testing.T) {
	if IsDev {
		if AppDirName != BaseName+"-Dev" {
			t.Fatalf("dev build: AppDirName = %q, want %q", AppDirName, BaseName+"-Dev")
		}
	} else {
		if AppDirName != BaseName {
			t.Fatalf("release build: AppDirName = %q, want %q", AppDirName, BaseName)
		}
	}
}
