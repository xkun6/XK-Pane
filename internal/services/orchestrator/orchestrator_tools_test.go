package orchestrator

import (
	"context"
	"net/http"
	"testing"
	"time"

	"xkpane/internal/storage"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// callerTool 从 RequestExtra.Header 读发起方 CLI；nil/缺失返回 ""。
func TestCallerTool(t *testing.T) {
	if callerTool(nil) != "" {
		t.Error("nil req 应返回 \"\"")
	}
	if callerTool(&mcp.CallToolRequest{}) != "" {
		t.Error("无 Extra 应返回 \"\"")
	}
	h := http.Header{}
	h.Set(callerToolHeader, "reclaude")
	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: h}}
	if got := callerTool(req); got != "reclaude" {
		t.Errorf("callerTool = %q, want reclaude", got)
	}
}

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		`C:\proj\a`:  "C:/proj/a",
		"/home/x/":   "/home/x",
		"/home/x//":  "/home/x",
		`C:\proj\a\`: "C:/proj/a",
		"already/ok": "already/ok",
	}
	for in, want := range cases {
		if got := normalizePath(in); got != want {
			t.Errorf("normalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBaseName(t *testing.T) {
	cases := map[string]string{
		`C:\proj\app`: "app",
		"/home/x/y/":  "y",
		"single":      "single",
	}
	for in, want := range cases {
		if got := baseName(in); got != want {
			t.Errorf("baseName(%q) = %q, want %q", in, got, want)
		}
	}
}

// computeEnterDelay：base 200ms；每 512 字节 +30ms；封顶 5s（忠实 cc-pane）。
func TestComputeEnterDelay(t *testing.T) {
	cases := []struct {
		textLen int
		want    time.Duration
	}{
		{0, 200 * time.Millisecond},
		{511, 200 * time.Millisecond},
		{512, 230 * time.Millisecond},
		{1024, 260 * time.Millisecond},
		{1 << 20, 5000 * time.Millisecond}, // 封顶
	}
	for _, c := range cases {
		if got := computeEnterDelay(c.textLen); got != c.want {
			t.Errorf("computeEnterDelay(%d) = %v, want %v", c.textLen, got, c.want)
		}
	}
}

// fakeProjects / fakeWorkspaces 用于白名单单测（不触 DB）。
type fakeProjects struct{ paths []string }

func (f fakeProjects) List(_ context.Context) ([]storage.Project, error) {
	out := make([]storage.Project, len(f.paths))
	for i, p := range f.paths {
		out[i] = storage.Project{Path: p}
	}
	return out, nil
}

type fakeWorkspaces struct{ projectPaths []string }

func (f fakeWorkspaces) List(_ context.Context) ([]storage.Workspace, error) {
	ps := make([]storage.WorkspaceProject, len(f.projectPaths))
	for i, p := range f.projectPaths {
		ps[i] = storage.WorkspaceProject{Path: p}
	}
	return []storage.Workspace{{Name: "ws1", Projects: ps}}, nil
}

func TestIsProjectRegistered(t *testing.T) {
	s := New(Deps{
		Projects:   fakeProjects{paths: []string{`C:\proj\db`}},
		Workspaces: fakeWorkspaces{projectPaths: []string{"/home/ws/p"}},
	})
	ctx := context.Background()
	// DB 项目命中（含路径分隔符归一化：传 / 仍命中存 \ 的项）。
	if !s.isProjectRegistered(ctx, "C:/proj/db") {
		t.Error("DB 项目应命中（归一化后比对）")
	}
	// 工作空间项目命中（尾斜杠归一化）。
	if !s.isProjectRegistered(ctx, "/home/ws/p/") {
		t.Error("工作空间项目应命中（去尾斜杠）")
	}
	// 未注册不命中。
	if s.isProjectRegistered(ctx, "/not/registered") {
		t.Error("未注册路径不应命中")
	}
}
