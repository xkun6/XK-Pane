package bedrockdocsync

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func blob(path string) giteaTreeEntry {
	sz := uint64(100)
	return giteaTreeEntry{Path: path, Kind: "blob", SHA: "x", Size: &sz}
}
func tree(path string) giteaTreeEntry {
	return giteaTreeEntry{Path: path, Kind: "tree", SHA: "0"}
}

func TestFilterBlobs(t *testing.T) {
	ext := map[string]bool{"md": true, "json": true}
	entries := []giteaTreeEntry{
		blob("mcguide/intro.md"),
		blob("mcguide/img/banner.png"), // 扩展 miss
		blob("mconline/api.json"),
		blob("docs/other.md"),    // 子目录 miss
		tree("mcguide/img"),      // tree 条目
		blob(".gitignore"),       // dotfile 无扩展
		blob("mcguide/notes.MD"), // 大写扩展
	}
	kept, skipped := filterBlobs(entries, []string{"mcguide", "mconline"}, ext)
	var paths []string
	for _, e := range kept {
		paths = append(paths, e.Path)
	}
	want := []string{"mcguide/intro.md", "mconline/api.json", "mcguide/notes.MD"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("kept=%v want %v", paths, want)
	}
	if skipped != 1 { // banner.png 子目录命中但扩展 miss
		t.Fatalf("skipped=%d want 1", skipped)
	}
}

func TestFilterBlobs_EmptySubdirsKeepsAll(t *testing.T) {
	ext := map[string]bool{"md": true}
	kept, _ := filterBlobs([]giteaTreeEntry{blob("README.md"), blob("data/x.md")}, nil, ext)
	if len(kept) != 2 {
		t.Fatalf("kept=%d want 2", len(kept))
	}
}

func TestFilterBlobs_EmptyExtKeepsAll(t *testing.T) {
	kept, skipped := filterBlobs([]giteaTreeEntry{blob("mcguide/x.md"), blob("mcguide/y.png")}, []string{"mcguide"}, map[string]bool{})
	if len(kept) != 2 || skipped != 0 {
		t.Fatalf("kept=%d skipped=%d want 2,0", len(kept), skipped)
	}
}

func TestGiteaSync_EndToEnd(t *testing.T) {
	blobBodies := map[string]string{
		"mcguide/intro.md":  "# intro",
		"mconline/api.json": "{}",
	}
	treeJSON := `{"sha":"deadbeef0000","tree":[` +
		`{"path":"mcguide/intro.md","type":"blob","sha":"1","size":42},` +
		`{"path":"mcguide/img/banner.png","type":"blob","sha":"2","size":42},` +
		`{"path":"mconline/api.json","type":"blob","sha":"3","size":42},` +
		`{"path":"README.md","type":"blob","sha":"4","size":42}` +
		`],"truncated":false}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/api/v1/repos/owner/repo":
			fmt.Fprint(w, `{"default_branch":"main"}`)
		case strings.Contains(p, "/git/trees/"):
			fmt.Fprint(w, treeJSON)
		case strings.Contains(p, "/raw/branch/main/"):
			rel := p[strings.Index(p, "/raw/branch/main/")+len("/raw/branch/main/"):]
			if body, ok := blobBodies[rel]; ok {
				fmt.Fprint(w, body)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	report, err := giteaSync(context.Background(), &http.Client{}, GiteaSyncParams{
		SourceName:   "wiki",
		APIBase:      srv.URL + "/api/v1",
		RawBase:      srv.URL,
		Owner:        "owner",
		Repo:         "repo",
		Subdirs:      []string{"mcguide", "mconline"},
		ExtWhitelist: []string{"md", "json"},
		TargetDir:    tmp,
		Concurrency:  2,
	}, NewProgressSink(nil))
	if err != nil {
		t.Fatalf("giteaSync: %v", err)
	}
	if report.Branch != "main" || report.TreeSHA != "deadbeef0000" {
		t.Fatalf("branch/sha: %+v", report)
	}
	if report.Fetched != 2 {
		t.Fatalf("fetched=%d want 2", report.Fetched)
	}
	if report.SkippedNotWhitelisted != 1 { // banner.png
		t.Fatalf("skipped=%d want 1", report.SkippedNotWhitelisted)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("failed: %+v", report.Failed)
	}
	if b, _ := os.ReadFile(filepath.Join(tmp, "mcguide", "intro.md")); string(b) != "# intro" {
		t.Fatalf("intro.md content: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(tmp, "mconline", "api.json")); string(b) != "{}" {
		t.Fatalf("api.json content: %q", b)
	}
	if _, err := os.Stat(filepath.Join(tmp, "mcguide", "img", "banner.png")); !os.IsNotExist(err) {
		t.Fatal("banner.png 不应被写入")
	}
}
