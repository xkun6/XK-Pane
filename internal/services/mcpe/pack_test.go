package mcpe

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, dir, uuid, moduleType string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	j := fmt.Sprintf(
		`{"format_version":1,"header":{"uuid":%q,"version":[1,0,0],"name":"test"},"modules":[{"type":%q,"uuid":"00000000-0000-0000-0000-000000000000","version":[1,0,0]}]}`,
		uuid, moduleType,
	)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(j), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseManifestBehavior(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "abc-uuid", "data")
	info, ok := parseManifest(filepath.Join(dir, "manifest.json"))
	if !ok {
		t.Fatal("expected ok")
	}
	if info.Type != PackBehavior {
		t.Errorf("type=%q want behavior", info.Type)
	}
	if info.UUID != "abc-uuid" {
		t.Errorf("uuid=%q", info.UUID)
	}
	if info.Version != "1.0.0" {
		t.Errorf("version=%q want 1.0.0", info.Version)
	}
}

func TestParseManifestResource(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "rp-uuid", "resources")
	info, ok := parseManifest(filepath.Join(dir, "manifest.json"))
	if !ok || info.Type != PackResource {
		t.Fatalf("expected resource, ok=%v type=%q", ok, info.Type)
	}
}

func TestParseManifestUnknownTypeReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "x", "scripts") // 未知 type
	if _, ok := parseManifest(filepath.Join(dir, "manifest.json")); ok {
		t.Fatal("unknown module type should not parse")
	}
}

func TestParseManifestMissingFile(t *testing.T) {
	if _, ok := parseManifest(filepath.Join(t.TempDir(), "nope", "manifest.json")); ok {
		t.Fatal("missing manifest should return false")
	}
}

func TestParsePackFindsRootManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "root-uuid", "data")
	m := map[PackType][]PackInfo{}
	if !parsePack(dir, m) {
		t.Fatal("expected found")
	}
	if len(m[PackBehavior]) != 1 {
		t.Fatalf("behavior count=%d", len(m[PackBehavior]))
	}
}

func TestParsePackFindsSubdirManifests(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, filepath.Join(dir, "bp"), "bp-uuid", "data")
	writeManifest(t, filepath.Join(dir, "rp"), "rp-uuid", "resources")
	m := map[PackType][]PackInfo{}
	if !parsePack(dir, m) {
		t.Fatal("expected found")
	}
	if len(m[PackBehavior]) != 1 || len(m[PackResource]) != 1 {
		t.Fatalf("bp=%d rp=%d", len(m[PackBehavior]), len(m[PackResource]))
	}
}

func TestParsePackEmptyReturnsFalse(t *testing.T) {
	m := map[PackType][]PackInfo{}
	if parsePack(t.TempDir(), m) {
		t.Fatal("empty dir should return false")
	}
	if len(m) != 0 {
		t.Fatal("map should stay empty")
	}
}

// BDS 存档布局：root/{behavior_packs,resource_packs}/<pack>/manifest.json
func TestParsePackFindsDepthTwoWorldLayout(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, filepath.Join(dir, "behavior_packs", "bp_inner"), "bp-uuid", "data")
	writeManifest(t, filepath.Join(dir, "resource_packs", "rp_inner"), "rp-uuid", "resources")
	m := map[PackType][]PackInfo{}
	if !parsePack(dir, m) {
		t.Fatal("expected found")
	}
	if len(m[PackBehavior]) != 1 || len(m[PackResource]) != 1 {
		t.Fatalf("bp=%d rp=%d", len(m[PackBehavior]), len(m[PackResource]))
	}
	if m[PackBehavior][0].UUID != "bp-uuid" || m[PackResource][0].UUID != "rp-uuid" {
		t.Fatalf("uuids: %q %q", m[PackBehavior][0].UUID, m[PackResource][0].UUID)
	}
}

// 深度 1 命中时不下钻：子目录里还有孙级 manifest 也不会被加进来（防回归）。
func TestParsePackDepthOneHitSkipsDescent(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, filepath.Join(dir, "bp"), "outer-uuid", "data")
	writeManifest(t, filepath.Join(dir, "bp", "nested"), "inner-uuid", "data")
	m := map[PackType][]PackInfo{}
	if !parsePack(dir, m) {
		t.Fatal("expected found")
	}
	if len(m[PackBehavior]) != 1 {
		t.Fatalf("behavior count=%d want 1", len(m[PackBehavior]))
	}
	if m[PackBehavior][0].UUID != "outer-uuid" {
		t.Fatalf("uuid=%q want outer-uuid", m[PackBehavior][0].UUID)
	}
}

// 守门测试：桥模组必须排在各类型列表最前，即 world_*_packs.json 里 Pack Stack [00]。
// 若退回 append（桥在业务包之后），业务 mod 一旦 import 崩溃就会中断 addon 加载链，
// 桥模组不再被 import —— 调试桥恰在最需要它的时刻失效。
func TestMergeBridgeFirstPutsBridgeAhead(t *testing.T) {
	packMap := map[PackType][]PackInfo{
		PackBehavior: {{UUID: "biz-beh", Version: "0.0.1"}},
		PackResource: {{UUID: "biz-res", Version: "0.0.1"}},
	}
	bridge := map[PackType][]PackInfo{
		PackBehavior: {{UUID: "183b0dc7", Version: "1.0.0"}},
	}
	mergeBridgeFirst(packMap, bridge)

	beh := packMap[PackBehavior]
	if len(beh) != 2 {
		t.Fatalf("behavior 应有 2 项，实际 %d", len(beh))
	}
	if beh[0].UUID != "183b0dc7" {
		t.Errorf("桥模组应排首位，实际首位=%q", beh[0].UUID)
	}
	if beh[1].UUID != "biz-beh" {
		t.Errorf("业务包应排其后，实际=%q", beh[1].UUID)
	}
	// 桥模组没有资源包条目 → 资源列表不得被改动。
	if got := packMap[PackResource]; len(got) != 1 || got[0].UUID != "biz-res" {
		t.Errorf("资源包列表不应被改动，实际=%+v", got)
	}
}

// 不得复用/污染入参 bridge 的底层数组（append 到 v 上会踩坏调用方切片）。
func TestMergeBridgeFirstDoesNotAliasInput(t *testing.T) {
	bridge := map[PackType][]PackInfo{
		PackBehavior: make([]PackInfo, 1, 8), // cap 充裕，append 会原地写
	}
	bridge[PackBehavior][0] = PackInfo{UUID: "183b0dc7"}
	packMap := map[PackType][]PackInfo{
		PackBehavior: {{UUID: "biz-beh"}},
	}
	mergeBridgeFirst(packMap, bridge)

	if len(bridge[PackBehavior]) != 1 || bridge[PackBehavior][0].UUID != "183b0dc7" {
		t.Errorf("入参 bridge 被修改: %+v", bridge[PackBehavior])
	}
}
