package bedrock

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── render 报告骨架（移植 mod.rs tests）──────────────────────────────────

func TestRenderValidateReport(t *testing.T) {
	r := renderValidateReport("Demo", 3, nil)
	if !strings.Contains(r, "Files scanned: 3") || !strings.Contains(r, "Issues: 0") || !strings.Contains(r, "## All checks passed") {
		t.Fatalf("no-issue 报告:\n%s", r)
	}
	issues := []issue{
		{issueInfo, "a.json", "info msg"},
		{issueWarning, "b.json", "warn msg"},
		{issueError, "c.json", "err msg"},
	}
	r2 := renderValidateReport("Demo", 3, issues)
	errPos := strings.Index(r2, "## Errors")
	warnPos := strings.Index(r2, "## Warnings")
	infoPos := strings.Index(r2, "## Infos")
	if !(errPos >= 0 && errPos < warnPos && warnPos < infoPos) {
		t.Fatalf("段顺序应 Errors<Warnings<Infos:\n%s", r2)
	}
}

// ── validate_test（移植 test_schema.rs tests）───────────────────────────

func TestValidateTest(t *testing.T) {
	valid := `{"name":"smoke","steps":[{"type":"wait","seconds":1},{"type":"assert","expr":"foo==1"},{"type":"action","command":"/say hi"},{"type":"screenshot"}]}`
	if r, _ := validateTest(valid); r != "VALID: 4 steps, all checks passed" {
		t.Fatalf("valid: %q", r)
	}
	if r, _ := validateTest("not json"); !strings.HasPrefix(r, "INVALID: JSON parse error") {
		t.Fatalf("parse error: %q", r)
	}
	if r, _ := validateTest(`{"steps":[]}`); !strings.Contains(r, "INVALID:") || !strings.Contains(r, "name: required field missing") {
		t.Fatalf("missing name: %q", r)
	}
	if r, _ := validateTest(`{"name":"t","steps":[{"type":"unknown"}]}`); !strings.Contains(r, `unknown type "unknown"`) || !strings.Contains(r, `["wait", "assert", "action", "screenshot"]`) {
		t.Fatalf("unknown type（含 {:?} 调试格式）: %q", r)
	}
	if r, _ := validateTest(`{"name":"t","steps":[{"type":"wait","seconds":"two"}]}`); !strings.Contains(r, `"seconds" must be a number`) {
		t.Fatalf("wait seconds: %q", r)
	}
	if r, _ := validateTest(`{"name":"t","steps":[{"type":"assert","expr":""}]}`); !strings.Contains(r, `"expr" must be a non-empty string`) {
		t.Fatalf("assert expr: %q", r)
	}
	if r, _ := validateTest(`{"name":"t","steps":[{"type":"action"}]}`); !strings.Contains(r, `missing "command" field`) {
		t.Fatalf("action command: %q", r)
	}
	if r, _ := validateTest(`{"name":"t","steps":["nope"]}`); !strings.Contains(r, "step 0: must be an object") {
		t.Fatalf("step scalar: %q", r)
	}
	if r, _ := validateTest("[]"); !strings.Contains(r, "top-level value must be a JSON object") {
		t.Fatalf("top-level: %q", r)
	}
	if r, _ := validateTest(`{"name":"snap","steps":[{"type":"screenshot","region":[0,0,100,100]}]}`); r != "VALID: 1 steps, all checks passed" {
		t.Fatalf("screenshot region: %q", r)
	}
}

// ── validate_json_definitions（移植 json_defs.rs tests）─────────────────

func TestValidateJSONDefinitions(t *testing.T) {
	if r, _ := validateJSONDefinitions("relative/dir"); !strings.Contains(r, "addon_dir must be an absolute path") {
		t.Fatalf("relative: %s", r)
	}
	ghost := filepath.Join(t.TempDir(), "does-not-exist")
	if r, _ := validateJSONDefinitions(ghost); !strings.Contains(r, "addon_dir does not exist") {
		t.Fatalf("missing: %s", r)
	}

	// happy path
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "netease_items_beh"))
	mustWrite(t, filepath.Join(root, "netease_items_beh", "good.json"),
		`{"format_version":"1.10","minecraft:item":{"description":{"identifier":"demo:hello"}}}`)
	r, _ := validateJSONDefinitions(root)
	if !strings.Contains(r, "Files scanned: 1") || !strings.Contains(r, "Errors: 0") || !strings.Contains(r, "Warnings: 0") {
		t.Fatalf("happy:\n%s", r)
	}

	// bom + parse + wrong version
	root2 := t.TempDir()
	mustMkdir(t, filepath.Join(root2, "netease_items_beh"))
	mustMkdir(t, filepath.Join(root2, "netease_blocks"))
	mustMkdir(t, filepath.Join(root2, "netease_recipes"))
	mustWrite(t, filepath.Join(root2, "netease_items_beh", "good.json"),
		`{"format_version":"1.10","minecraft:item":{"description":{"identifier":"demo:x"}}}`)
	mustWrite(t, filepath.Join(root2, "netease_items_beh", "bad.json"),
		`{"format_version":"2.0","minecraft:item":{"description":{"identifier":"demo:y"}}}`)
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"format_version":"1.10.0","minecraft:block":{"description":{"identifier":"demo:z"}}}`)...)
	if err := os.WriteFile(filepath.Join(root2, "netease_blocks", "bom.json"), bom, 0o644); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root2, "netease_recipes", "parse_err.json"), "{ not json }")
	r2, _ := validateJSONDefinitions(root2)
	for _, want := range []string{"Files scanned: 4", "JSON parse error", "UTF-8 BOM", `expects "1.10"`} {
		if !strings.Contains(r2, want) {
			t.Fatalf("缺 %q:\n%s", want, r2)
		}
	}

	// minecraft namespace
	root3 := t.TempDir()
	mustMkdir(t, filepath.Join(root3, "netease_items_beh"))
	mustWrite(t, filepath.Join(root3, "netease_items_beh", "override.json"),
		`{"format_version":"1.10","minecraft:item":{"description":{"identifier":"minecraft:stick"}}}`)
	if r3, _ := validateJSONDefinitions(root3); !strings.Contains(r3, "override is allowed but discouraged") {
		t.Fatalf("minecraft ns:\n%s", r3)
	}

	// missing namespace separator
	root4 := t.TempDir()
	mustMkdir(t, filepath.Join(root4, "netease_items_beh"))
	mustWrite(t, filepath.Join(root4, "netease_items_beh", "noid.json"),
		`{"format_version":"1.10","minecraft:item":{"description":{"identifier":"bareword"}}}`)
	if r4, _ := validateJSONDefinitions(root4); !strings.Contains(r4, "missing namespace prefix") {
		t.Fatalf("missing separator:\n%s", r4)
	}
}

// ── validate_addon_assets（移植 assets.rs tests）────────────────────────

func makePNG(w, h uint32) []byte {
	v := make([]byte, 0, 33)
	v = append(v, pngMagic...)
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], 13)
	v = append(v, lenBuf[:]...)
	v = append(v, []byte("IHDR")...)
	var wb, hb [4]byte
	binary.BigEndian.PutUint32(wb[:], w)
	binary.BigEndian.PutUint32(hb[:], h)
	v = append(v, wb[:]...)
	v = append(v, hb[:]...)
	v = append(v, 8, 6, 0, 0, 0, 0, 0, 0, 0)
	return v
}

func TestValidateAddonAssets(t *testing.T) {
	if r, _ := validateAddonAssets("relative"); !strings.Contains(r, "must be an absolute path") {
		t.Fatalf("relative: %s", r)
	}
	// missing textures dir
	if r, _ := validateAddonAssets(t.TempDir()); !strings.Contains(r, "no textures/ directory") || !strings.Contains(r, "Files scanned: 0") {
		t.Fatalf("missing textures: %s", r)
	}

	// happy clean png
	root := t.TempDir()
	blocks := filepath.Join(root, "textures", "blocks")
	mustMkdir(t, blocks)
	mustWriteBytes(t, filepath.Join(blocks, "grass.png"), makePNG(16, 16))
	r, _ := validateAddonAssets(root)
	if !strings.Contains(r, "Files scanned: 1") || !strings.Contains(r, "Errors: 0") || !strings.Contains(r, "Warnings: 0") {
		t.Fatalf("happy png:\n%s", r)
	}

	// uppercase filename + NPOT
	root2 := t.TempDir()
	items := filepath.Join(root2, "textures", "items")
	mustMkdir(t, items)
	mustWriteBytes(t, filepath.Join(items, "Sword.png"), makePNG(33, 33))
	r2, _ := validateAddonAssets(root2)
	if !strings.Contains(r2, "uppercase characters") || !strings.Contains(r2, "not power-of-two") {
		t.Fatalf("uppercase+npot:\n%s", r2)
	}

	// unsupported format + non-image skipped
	root3 := t.TempDir()
	blocks3 := filepath.Join(root3, "textures", "blocks")
	mustMkdir(t, blocks3)
	mustWriteBytes(t, filepath.Join(blocks3, "oops.jpg"), []byte{0xFF, 0xD8, 0xFF})
	mustWrite(t, filepath.Join(blocks3, "note.txt"), "ignore me")
	r3, _ := validateAddonAssets(root3)
	if !strings.Contains(r3, "Files scanned: 1") || !strings.Contains(r3, "unsupported image format") {
		t.Fatalf("unsupported:\n%s", r3)
	}

	// tga skipped for dim check
	root4 := t.TempDir()
	items4 := filepath.Join(root4, "textures", "items")
	mustMkdir(t, items4)
	mustWriteBytes(t, filepath.Join(items4, "sword.tga"), make([]byte, 18))
	r4, _ := validateAddonAssets(root4)
	if !strings.Contains(r4, "Files scanned: 1") || strings.Contains(r4, "not power-of-two") || strings.Contains(r4, "unsupported image format") {
		t.Fatalf("tga:\n%s", r4)
	}

	// corrupt png header
	root5 := t.TempDir()
	blocks5 := filepath.Join(root5, "textures", "blocks")
	mustMkdir(t, blocks5)
	mustWriteBytes(t, filepath.Join(blocks5, "bad.png"), make([]byte, 8))
	if r5, _ := validateAddonAssets(root5); !strings.Contains(r5, "PNG header missing or corrupt") {
		t.Fatalf("corrupt png:\n%s", r5)
	}

	// aspect ratio info for power-of-two but odd shape
	root6 := t.TempDir()
	blocks6 := filepath.Join(root6, "textures", "blocks")
	mustMkdir(t, blocks6)
	mustWriteBytes(t, filepath.Join(blocks6, "stripe.png"), makePNG(16, 64))
	r6, _ := validateAddonAssets(root6)
	if !strings.Contains(r6, "aspect ratio 16:64") || strings.Contains(r6, "not power-of-two") {
		t.Fatalf("aspect:\n%s", r6)
	}
}

func mustWriteBytes(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
