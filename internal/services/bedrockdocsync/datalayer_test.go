package bedrockdocsync

import (
	"encoding/json"
	"strings"
	"testing"

	"xkpane/internal/events"
)

// ── manifest（移植 manifest.rs tests）──────────────────────────────────

func TestManifest_Roundtrip(t *testing.T) {
	orig := Manifest{
		SchemaVersion: 1,
		SyncedAt:      "2026-05-17T12:34:56Z",
		Sources: map[string]SourceInfo{
			"mcguide":  GiteaSource(DefaultWikiRemote, "main", "abc123", "2026-05-17T12:00:00Z"),
			"mcdocs":   HTTPSource("https://mc.163.com/dev/", 1843, "2026-05-17T12:34:56Z"),
			"mconline": EmbeddedSource(),
		},
		Stats: ManifestStats{APIEntries: 1936, TemplateEntries: 42, GuideFiles: 1526},
	}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, want := range []string{`"kind":"gitea"`, `"kind":"http"`, `"kind":"embedded"`, `"treeSha":"abc123"`} {
		if !strings.Contains(js, want) {
			t.Fatalf("缺 %q:\n%s", want, js)
		}
	}
	var dec Manifest
	if err := json.Unmarshal(b, &dec); err != nil {
		t.Fatal(err)
	}
	if dec.SchemaVersion != 1 || len(dec.Sources) != 3 || dec.Stats.APIEntries != 1936 {
		t.Fatalf("往返不符: %+v", dec)
	}
	if dec.Sources["mcguide"].Kind != "gitea" || dec.Sources["mcguide"].TreeSHA != "abc123" {
		t.Fatalf("mcguide: %+v", dec.Sources["mcguide"])
	}
	if dec.Sources["mcdocs"].Kind != "http" || dec.Sources["mcdocs"].FileCount != 1843 {
		t.Fatalf("mcdocs: %+v", dec.Sources["mcdocs"])
	}
	if dec.Sources["mconline"].Kind != "embedded" {
		t.Fatalf("mconline: %+v", dec.Sources["mconline"])
	}
}

func TestSyncSourcesConfig_Effective(t *testing.T) {
	// 默认（空）→ 官方默认。
	var def SyncSourcesConfig
	if def.EffectiveMcguideRemote() != DefaultWikiRemote || def.EffectiveMcguideBranch() != DefaultWikiBranch ||
		def.EffectiveMconlineRemote() != DefaultWikiRemote || def.EffectiveMcdocsBaseURL() != DefaultMcdocsBaseURL {
		t.Fatal("默认应回退官方")
	}
	// 空白字符串当默认。
	ws := SyncSourcesConfig{McguideRemote: "", McguideBranch: "   ", McdocsBaseURL: " \t"}
	if ws.EffectiveMcguideRemote() != DefaultWikiRemote || ws.EffectiveMcguideBranch() != DefaultWikiBranch || ws.EffectiveMcdocsBaseURL() != DefaultMcdocsBaseURL {
		t.Fatal("空白应当默认")
	}
	// 覆盖生效。
	ov := SyncSourcesConfig{McguideRemote: "https://gitea.example/fork/wiki", McguideBranch: "dev"}
	if ov.EffectiveMcguideRemote() != "https://gitea.example/fork/wiki" || ov.EffectiveMcguideBranch() != "dev" {
		t.Fatal("覆盖应生效")
	}
	if ov.EffectiveMconlineRemote() != DefaultWikiRemote {
		t.Fatal("未覆盖的 mconline 应仍默认")
	}
}

// ── progress（移植 progress.rs tests）──────────────────────────────────

func TestProgressSink(t *testing.T) {
	var got []events.BedrockDocSyncProgressPayload
	sink := NewProgressSink(func(e events.BedrockDocSyncProgressPayload) { got = append(got, e) })

	sink.Started([]string{"mcguide", "mcdocs"})
	sink.GiteaSync("wiki", "listing_tree")
	sink.HTTPFetch("mcdocs", 42, 100)
	sink.Info("a")
	sink.Warn("b")
	sink.ErrorLog("c")

	if len(got) != 6 {
		t.Fatalf("应收 6 事件，得 %d", len(got))
	}
	if got[0].Stage != "started" || len(got[0].Sources) != 2 {
		t.Fatalf("started: %+v", got[0])
	}
	if got[1].Stage != "gitea_sync" || got[1].Status != "listing_tree" {
		t.Fatalf("gitea_sync: %+v", got[1])
	}
	if got[2].Stage != "http_fetch" || got[2].Fetched != 42 || got[2].Total != 100 {
		t.Fatalf("http_fetch: %+v", got[2])
	}
	if got[3].Level != "info" || got[4].Level != "warn" || got[5].Level != "error" {
		t.Fatalf("log levels: %+v %+v %+v", got[3], got[4], got[5])
	}
	// stage 值序列化到 JSON。
	b, _ := json.Marshal(got[1])
	if !strings.Contains(string(b), `"stage":"gitea_sync"`) {
		t.Fatalf("stage 序列化: %s", b)
	}
}

func TestProgressSink_NoopNoPanic(t *testing.T) {
	sink := NewProgressSink(nil)
	sink.Cancelled() // 不应 panic
	sink.Info("orphan")
	sink.Failed("boom", true)
}

// ── proxy（移植 proxy.rs from_env tests）───────────────────────────────

func TestProxyConfigFromEnvOr(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://from-env:9999")

	// 显式非空 URL 优先于 env。
	explicit := &ProxyConfig{HTTPSProxy: "http://explicit:7890", Auth: "u:p"}
	if r := ProxyConfigFromEnvOr(explicit); r.HTTPSProxy != "http://explicit:7890" || r.Auth != "u:p" {
		t.Fatalf("显式应优先: %+v", r)
	}
	// 空显式 URL → 查 env。
	if r := ProxyConfigFromEnvOr(&ProxyConfig{HTTPSProxy: ""}); r.HTTPSProxy != "http://from-env:9999" {
		t.Fatalf("空显式应查 env: %+v", r)
	}
	// nil 显式 → 查 env。
	if r := ProxyConfigFromEnvOr(nil); r.HTTPSProxy != "http://from-env:9999" {
		t.Fatalf("nil 应查 env: %+v", r)
	}
}

func TestProxyConfig_HTTPClient(t *testing.T) {
	// 默认（无代理）应能建 client。
	if _, err := (ProxyConfig{}).HTTPClient(); err != nil {
		t.Fatalf("默认 client: %v", err)
	}
	// 非法代理 url 应报错。
	if _, err := (ProxyConfig{HTTPSProxy: "not a url at all"}).HTTPClient(); err == nil {
		t.Fatal("非法代理应报错")
	}
}
