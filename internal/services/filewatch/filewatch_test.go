package filewatch

import (
	"context"
	"path/filepath"
	"testing"

	"xkpane/internal/events"
)

// 监听的 fsnotify emit 行为依赖系统通知 + 时序，留真机手测；此处只覆盖确定性的生命周期 API。

func TestWatchLifecycle(t *testing.T) {
	s := New()
	ctx := context.Background()

	// 空 root = 仅停止，不报错。
	if err := s.Watch(ctx, ""); err != nil {
		t.Fatalf("Watch(\"\") 期望 nil，得 %v", err)
	}

	// 不存在的目录应报错。
	if err := s.Watch(ctx, filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("Watch(不存在目录) 期望报错")
	}

	// 监听临时目录应成功；切换到另一个根应停旧起新；Unwatch 幂等。
	if err := s.Watch(ctx, t.TempDir()); err != nil {
		t.Fatalf("Watch(tempdir): %v", err)
	}
	if err := s.Watch(ctx, t.TempDir()); err != nil {
		t.Fatalf("Watch(切换根): %v", err)
	}
	s.Unwatch(ctx)
	s.Unwatch(ctx) // 二次调用安全
}

func TestSetEmitterNilSafe(t *testing.T) {
	s := New()
	// 未设 emitter 时 emitChanged 不应 panic。
	s.emitChanged(events.FileChangedPayload{Root: "x", Dirs: []string{"x"}})
}
