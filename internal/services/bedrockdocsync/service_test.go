package bedrockdocsync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"xkpane/internal/events"
)

// hookRec 记录 BedrockHooks 回调，供断言 SetDataRoot 时机 / 注入 RuntimeStats。
type hookRec struct {
	mu        sync.Mutex
	rootCalls []string
	stats     ManifestStats
}

func (h *hookRec) hooks() BedrockHooks {
	return BedrockHooks{
		SetDataRoot: func(root string) {
			h.mu.Lock()
			h.rootCalls = append(h.rootCalls, root)
			h.mu.Unlock()
		},
		RuntimeStats: func() ManifestStats {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.stats
		},
	}
}

func (h *hookRec) lastRoot() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.rootCalls) == 0 {
		return "", false
	}
	return h.rootCalls[len(h.rootCalls)-1], true
}

// newTestService 建一个 dataDir 在临时目录、注入 stub hooks 的服务。
func newTestService(t *testing.T) (*Service, *hookRec) {
	t.Helper()
	rec := &hookRec{stats: ManifestStats{APIEntries: 1900, TemplateEntries: 42, GuideFiles: 1800}}
	return New(t.TempDir(), rec.hooks()), rec
}

// capture 收集进度事件（emitter 可能被 worker goroutine 并发调用，故加锁）。
type capture struct {
	mu  sync.Mutex
	evs []events.BedrockDocSyncProgressPayload
}

func (c *capture) attach(s *Service) {
	s.SetEmitter(func(name string, payload any) {
		if p, ok := payload.(events.BedrockDocSyncProgressPayload); ok {
			c.mu.Lock()
			c.evs = append(c.evs, p)
			c.mu.Unlock()
		}
	})
}

func (c *capture) snapshot() []events.BedrockDocSyncProgressPayload {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]events.BedrockDocSyncProgressPayload(nil), c.evs...)
}

// ── parseGiteaURL ───────────────────────────────────────────────────────

func TestParseGiteaURL_Canonical(t *testing.T) {
	api, raw, owner, repo, err := parseGiteaURL("https://git.mcplugin.top/xiaokun/netease-bedrock-wiki")
	if err != nil {
		t.Fatal(err)
	}
	if api != "https://git.mcplugin.top/api/v1" || raw != "https://git.mcplugin.top" || owner != "xiaokun" || repo != "netease-bedrock-wiki" {
		t.Fatalf("解析异常: api=%q raw=%q owner=%q repo=%q", api, raw, owner, repo)
	}
}

func TestParseGiteaURL_StripsDotGitAndSlash(t *testing.T) {
	_, _, owner, repo, err := parseGiteaURL("https://git.example.com/foo/bar.git/")
	if err != nil || owner != "foo" || repo != "bar" {
		t.Fatalf("owner=%q repo=%q err=%v", owner, repo, err)
	}
	_, _, _, repo2, _ := parseGiteaURL("https://git.example.com/foo/bar.git")
	if repo2 != "bar" {
		t.Fatalf("repo=%q", repo2)
	}
}

func TestParseGiteaURL_PreservesSchemeAndPort(t *testing.T) {
	api, raw, _, _, err := parseGiteaURL("http://gitea.local:3000/team/repo")
	if err != nil || raw != "http://gitea.local:3000" || api != "http://gitea.local:3000/api/v1" {
		t.Fatalf("raw=%q api=%q err=%v", raw, api, err)
	}
}

func TestParseGiteaURL_RejectsGitHub(t *testing.T) {
	if _, _, _, _, err := parseGiteaURL("https://github.com/foo/bar"); err == nil {
		t.Fatal("github.com 应被拒绝")
	}
	if _, _, _, _, err := parseGiteaURL("https://GITHUB.COM/foo/bar"); err == nil {
		t.Fatal("GITHUB.COM 大小写不敏感应被拒绝")
	}
}

func TestParseGiteaURL_RejectsMalformed(t *testing.T) {
	for _, u := range []string{"not a url", "https://", "https://host", "https://host/owner"} {
		if _, _, _, _, err := parseGiteaURL(u); err == nil {
			t.Fatalf("%q 应返错", u)
		}
	}
}

// ── dir size ────────────────────────────────────────────────────────────

func TestDirSizeMB_HandlesMissing(t *testing.T) {
	if got := dirSizeMB(filepath.Join(t.TempDir(), "nope")); got != 0.0 {
		t.Fatalf("缺失路径应 0，得 %v", got)
	}
}

// ── status ──────────────────────────────────────────────────────────────

func TestStatus_NoData(t *testing.T) {
	svc, _ := newTestService(t)
	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.UserDataPresent {
		t.Fatal("无 bedrock-data 应 userDataPresent=false")
	}
	if st.Manifest != nil {
		t.Fatal("无 manifest 应 nil")
	}
	if st.DataSizeMB != 0.0 {
		t.Fatalf("dataSizeMb 应 0，得 %v", st.DataSizeMB)
	}
	if len(st.Archives) != 0 {
		t.Fatalf("无归档应空，得 %v", st.Archives)
	}
	// runtime_stats 透传自 hook（首启即真实数）。
	if st.RuntimeStats.APIEntries != 1900 {
		t.Fatalf("runtimeStats 应透传 hook：%+v", st.RuntimeStats)
	}
}

func TestStatus_RoundTripsManifest(t *testing.T) {
	svc, _ := newTestService(t)
	bd := svc.bedrockDataDir()
	must(t, os.MkdirAll(bd, 0o755))
	m := Manifest{
		SchemaVersion: 1,
		SyncedAt:      "2026-05-17T00:00:00Z",
		Sources:       map[string]SourceInfo{"mcguide": GiteaSource("r", "main", "abc", "2026-05-17T00:00:00Z")},
		Stats:         ManifestStats{APIEntries: 7},
	}
	data, _ := marshalIndentNoEscape(m)
	mustWrite(t, filepath.Join(bd, "manifest.json"), string(data))

	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.UserDataPresent {
		t.Fatal("应 userDataPresent=true")
	}
	if st.Manifest == nil || st.Manifest.Stats.APIEntries != 7 {
		t.Fatalf("manifest 回读异常：%+v", st.Manifest)
	}
	if got := st.Manifest.Sources["mcguide"].Kind; got != "gitea" {
		t.Fatalf("source kind 应 gitea，得 %q", got)
	}
}

// ── reset / archives ────────────────────────────────────────────────────

func TestReset_ErrorsWhenNoData(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.Reset(); err == nil {
		t.Fatal("无数据 reset 应返错")
	}
}

func TestReset_ArchivesNotDeletes(t *testing.T) {
	svc, rec := newTestService(t)
	bd := svc.bedrockDataDir()
	must(t, os.MkdirAll(bd, 0o755))
	mustWrite(t, filepath.Join(bd, "sentinel.txt"), "hello")

	report, err := svc.Reset()
	if err != nil {
		t.Fatal(err)
	}
	if !pathExists(report.ArchivedTo) || !pathExists(filepath.Join(report.ArchivedTo, "sentinel.txt")) {
		t.Fatalf("归档目录/哨兵文件丢失：%s", report.ArchivedTo)
	}
	if pathExists(bd) {
		t.Fatal("原 bedrock-data 应已移走")
	}
	// reset 应清 data root（SetDataRoot("")）。
	if root, ok := rec.lastRoot(); !ok || root != "" {
		t.Fatalf("reset 应 SetDataRoot(\"\")，得 %q ok=%v", root, ok)
	}
}

func TestListArchives_SortsNewestFirst(t *testing.T) {
	svc, _ := newTestService(t)
	for _, stamp := range []string{"20240101-000000", "20260517-120000", "20250101-000000"} {
		must(t, os.MkdirAll(filepath.Join(svc.dataDir, "bedrock-data.archive-"+stamp), 0o755))
	}
	must(t, os.MkdirAll(filepath.Join(svc.dataDir, "unrelated"), 0o755))

	archives, err := svc.ListArchives()
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 3 {
		t.Fatalf("应 3 个归档，得 %d", len(archives))
	}
	if archives[0].CreatedAt != "20260517-120000" || archives[1].CreatedAt != "20250101-000000" || archives[2].CreatedAt != "20240101-000000" {
		t.Fatalf("未按新到旧排序：%+v", archives)
	}
}

func TestPruneOldArchives_KeepsN(t *testing.T) {
	svc, _ := newTestService(t)
	for _, stamp := range []string{
		"20240101-000000", "20250101-000000", "20260101-000000", "20260201-000000",
		"20260301-000000", "20260401-000000", "20260501-000000",
	} {
		must(t, os.MkdirAll(filepath.Join(svc.dataDir, "bedrock-data.archive-"+stamp), 0o755))
	}
	if got, _ := svc.ListArchives(); len(got) != 7 {
		t.Fatalf("应 7 个，得 %d", len(got))
	}
	svc.pruneOldArchives(3)
	remaining, _ := svc.ListArchives()
	if len(remaining) != 3 {
		t.Fatalf("prune 后应剩 3，得 %d", len(remaining))
	}
	if remaining[0].CreatedAt != "20260501-000000" || remaining[2].CreatedAt != "20260301-000000" {
		t.Fatalf("应保留最新 3 个：%+v", remaining)
	}
}

// ── proxy / sources 持久化 ───────────────────────────────────────────────

func TestSetProxy_RejectsUnreachable(t *testing.T) {
	svc, _ := newTestService(t)
	// 故意给一个连不上的代理：probe 经它 CONNECT github.com 立即失败（不真连 github.com）。
	result, err := svc.SetProxy(context.Background(), ProxyConfig{HTTPSProxy: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("SetProxy 应返 Ok+探测结果，得 err=%v", err)
	}
	if result.OK {
		t.Fatal("坏代理不应探测成功")
	}
	if pathExists(svc.proxyConfigPath()) {
		t.Fatal("探测失败不应持久化 proxy.json")
	}
}

func TestGetProxy_EmptyWhenAbsent(t *testing.T) {
	svc, _ := newTestService(t)
	if p := svc.GetProxy(); p.HTTPSProxy != "" || p.Auth != "" {
		t.Fatalf("无持久化应返零值，得 %+v", p)
	}
}

func TestSources_RoundTripAndFallbackPath(t *testing.T) {
	// bedrock-data 不存在 → 落数据根旁的 bedrock-data.sources.json。
	svc, _ := newTestService(t)
	if pathExists(svc.bedrockDataDir()) {
		t.Fatal("前置：bedrock-data 不应存在")
	}
	cfg := SyncSourcesConfig{McguideRemote: "https://example.com/foo.git", McguideBranch: "dev"}
	if err := svc.SetSources(cfg); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(svc.dataDir, "bedrock-data.sources.json")
	if !pathExists(fallback) {
		t.Fatalf("bedrock-data 缺失时应落 fallback 路径：%s", fallback)
	}
	got := svc.GetSources()
	if got.McguideRemote != "https://example.com/foo.git" || got.McguideBranch != "dev" {
		t.Fatalf("回读异常：%+v", got)
	}

	// bedrock-data 存在 → 落其内。
	must(t, os.MkdirAll(svc.bedrockDataDir(), 0o755))
	if err := svc.SetSources(SyncSourcesConfig{MconlineRemote: "https://in-tree/wiki"}); err != nil {
		t.Fatal(err)
	}
	if !pathExists(filepath.Join(svc.bedrockDataDir(), "sources.json")) {
		t.Fatal("bedrock-data 存在时应落其内 sources.json")
	}
}

func TestGetSources_EmptyWhenAbsent(t *testing.T) {
	svc, _ := newTestService(t)
	if c := svc.GetSources(); c.McguideRemote != "" || c.McdocsBaseURL != "" {
		t.Fatalf("无持久化应返零值，得 %+v", c)
	}
}

// ── sync 编排 ───────────────────────────────────────────────────────────

func TestSync_CancellationAbortsEarly(t *testing.T) {
	svc, _ := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预取消

	opts := SyncOptions{Sources: []SourceSelector{SelMcdocs}, Concurrency: u32ptr(1)}
	if _, err := svc.Sync(ctx, opts); err == nil {
		t.Fatal("预取消 sync 应返错")
	}
}

func TestSync_EmptySourcesUsesDefaults(t *testing.T) {
	svc, _ := newTestService(t)
	cap := &capture{}
	cap.attach(svc)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预取消，仅验 Started 的默认源

	_, _ = svc.Sync(ctx, SyncOptions{}) // 空 opts → 默认 [mcguide, mconline]

	evs := cap.snapshot()
	if len(evs) == 0 || evs[0].Stage != "started" {
		t.Fatalf("首个事件应 started：%+v", evs)
	}
	want := []string{"mcguide", "mconline"}
	if len(evs[0].Sources) != 2 || evs[0].Sources[0] != want[0] || evs[0].Sources[1] != want[1] {
		t.Fatalf("默认源应 [mcguide, mconline]，得 %v", evs[0].Sources)
	}
}

func TestSync_McdocsFallbackWhenUnreachable(t *testing.T) {
	// 强制无代理（不受开发机 env 影响），mcdocs 指向不可达端口 → 回退 Embedded，但 sync 仍完成。
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		t.Setenv(k, "")
	}
	svc, rec := newTestService(t)
	cap := &capture{}
	cap.attach(svc)

	opts := SyncOptions{
		Sources:       []SourceSelector{SelMcdocs},
		Concurrency:   u32ptr(1),
		McdocsBaseURL: "http://127.0.0.1:1/never/listens/",
	}
	report, err := svc.Sync(context.Background(), opts)
	if err != nil {
		t.Fatalf("sync 应完成（仅 mcdocs 回退）：%v", err)
	}
	if len(report.SourcesSynced) != 0 {
		t.Fatalf("不应有源同步成功：%v", report.SourcesSynced)
	}
	found := false
	for _, s := range report.SourcesSkipped {
		if s.Source == "mcdocs" {
			found = true
		}
	}
	if !found {
		t.Fatalf("mcdocs 应在 skipped：%+v", report.SourcesSkipped)
	}

	// 应发过 Completed 事件。
	sawCompleted := false
	for _, e := range cap.snapshot() {
		if e.Stage == "completed" {
			sawCompleted = true
		}
	}
	if !sawCompleted {
		t.Fatal("应发 completed 事件")
	}

	// 换目录后应 SetDataRoot 到 live 目录。
	if root, ok := rec.lastRoot(); !ok || root != svc.bedrockDataDir() {
		t.Fatalf("应 SetDataRoot 到 %q，得 %q ok=%v", svc.bedrockDataDir(), root, ok)
	}

	// manifest 应记录 mcdocs = Embedded。
	manifestPath := filepath.Join(svc.bedrockDataDir(), "manifest.json")
	raw, rerr := os.ReadFile(manifestPath)
	if rerr != nil {
		t.Fatalf("manifest 应存在：%v", rerr)
	}
	var parsed Manifest
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("manifest 解析失败：%v", err)
	}
	if parsed.Sources["mcdocs"].Kind != "embedded" {
		t.Fatalf("mcdocs 应记录 embedded，得 %q", parsed.Sources["mcdocs"].Kind)
	}
	// manifest.stats 来自 runtime hook。
	if parsed.Stats.APIEntries != 1900 {
		t.Fatalf("manifest stats 应透传 hook，得 %+v", parsed.Stats)
	}
}

func u32ptr(v uint32) *uint32 { return &v }
