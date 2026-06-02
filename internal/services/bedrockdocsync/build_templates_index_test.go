package bedrockdocsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateTags_MatchesRules(t *testing.T) {
	tags := generateTags("传送门")
	for _, want := range []string{"传送门", "维度", "方块交互"} {
		if !containsStr(tags, want) {
			t.Fatalf("应含 %q：%v", want, tags)
		}
	}
	// 字典序升序（复刻 BTreeSet → Vec）。
	for i := 1; i < len(tags); i++ {
		if tags[i-1] > tags[i] {
			t.Fatalf("tags 非字典序：%v", tags)
		}
	}
}

func TestGenerateTags_FallsBackToName(t *testing.T) {
	tags := generateTags("不在规则字典中的项目")
	if !reflect.DeepEqual(tags, []string{"不在规则字典中的项目"}) {
		t.Fatalf("无命中应回退项目名：%v", tags)
	}
}

func TestFindProjects_RequiresBehaviorSubdir(t *testing.T) {
	dir := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(dir, "ProjectA", "MyBehaviorPack"), 0o755))
	must(t, os.MkdirAll(filepath.Join(dir, "ProjectB", "README"), 0o755))
	must(t, os.MkdirAll(filepath.Join(dir, "ProjectC", "foo_behavior"), 0o755))

	projects := findProjects(dir)
	var names []string
	for _, p := range projects {
		names = append(names, p[0])
	}
	if !containsStr(names, "ProjectA") || !containsStr(names, "ProjectC") {
		t.Fatalf("应含 ProjectA/ProjectC：%v", names)
	}
	if containsStr(names, "ProjectB") {
		t.Fatalf("ProjectB 无 behav 子目录，不应入选：%v", names)
	}
	// findProjects 应按目录名升序。
	if len(projects) >= 2 && !(projects[0][0] <= projects[1][0]) {
		t.Fatalf("项目未按名升序：%v", names)
	}
}

func TestScanListenForEvent_FindsThirdArg(t *testing.T) {
	src := `
def init(self):
    serverApi.GetSystem(modConfig.Namespace, modConfig.SystemName).ListenForEvent(
        serverApi.GetEngineNamespace(),
        serverApi.GetEngineSystemName(),
        "AddServerPlayerEvent",
        self,
        self.OnPlayerAdd
    )
`
	out := map[string]bool{}
	scanListenForEvent(src, out)
	if !out["AddServerPlayerEvent"] {
		t.Fatalf("应捕获 AddServerPlayerEvent：%v", out)
	}
}

func TestScanEventLiterals_OnlyEventSuffix(t *testing.T) {
	out := map[string]bool{}
	scanEventLiterals(`x = "MyCustomEvent"; y = "NotAnEventX"; z = "with space Event"`, out)
	if !out["MyCustomEvent"] {
		t.Fatalf("应捕获 MyCustomEvent：%v", out)
	}
	if out["NotAnEventX"] {
		t.Fatalf("不以 Event 结尾不应捕获：%v", out)
	}
	if out["with space Event"] {
		t.Fatalf("含空格非裸标识符不应捕获：%v", out)
	}
}

func TestScanCreateComp_CaptursIdentifier(t *testing.T) {
	src := `
self.compFactory = serverApi.GetEngineCompFactory()
block = self.compFactory.CreateBlockInfo(playerId)
item = serverApi.GetEngineCompFactory().CreateItem(playerId)
`
	components := map[string]bool{}
	scanCreateComp(src, "compFactory.Create", components)
	scanCreateComp(src, "GetEngineCompFactory().Create", components)
	if !components["BlockInfo"] || !components["Item"] {
		t.Fatalf("应含 BlockInfo/Item：%v", components)
	}
}

func TestScanCompMethods_WordBoundaryAndExclude(t *testing.T) {
	src := `
        self.comp.SetPosition(pos)
        block = self.compFactory.CreateBlockInfo(x)
        comp.GetData()
        comp.Get(key)
`
	out := map[string]bool{}
	scanCompMethods(src, out, excludeMethods)
	if !out["SetPosition"] {
		t.Fatalf("应含 SetPosition：%v", out)
	}
	// GetData 是完整方法名，排除集是精确匹配（只排 "Get" 裸名），故 GetData 应被捕获。
	if !out["GetData"] {
		t.Fatalf("GetData 非排除名应被捕获：%v", out)
	}
	// CreateBlockInfo 前是 compFactory：'.' 前 4 字符为 "tory" 不以 comp/Comp 结尾，不应命中。
	if out["CreateBlockInfo"] {
		t.Fatalf("compFactory.Create 不应被 comp-method 扫描命中：%v", out)
	}
	// 裸 "Get" 精确命中排除集。
	if out["Get"] {
		t.Fatalf("裸 Get 应被排除：%v", out)
	}
}

func TestScanRegisterSystem_ParsesThreeArgs(t *testing.T) {
	src := `
serverApi.RegisterSystem("MyMod", "MySystem", "MyMod.scripts.MySystem.MySystem")
clientApi.RegisterSystem("MyMod", "MyClient", "MyMod.scripts.MyClient.MyClient")
`
	var out []systemEntry
	scanRegisterSystem(src, &out)
	if len(out) != 2 {
		t.Fatalf("应 2 条，得 %d：%v", len(out), out)
	}
	if out[0].Namespace != "MyMod" || out[0].Name != "MySystem" || out[0].ClsPath != "MyMod.scripts.MySystem.MySystem" {
		t.Fatalf("首条异常：%+v", out[0])
	}
}

func TestFindModBinding(t *testing.T) {
	name, ok := findModBinding(`@Mod.Binding(name="PortalMod", version="0.0.1")`)
	if !ok || name != "PortalMod" {
		t.Fatalf("应得 PortalMod：%q ok=%v", name, ok)
	}
	if _, ok := findModBinding(`@Mod.Binding(version="0.0.1")`); ok {
		t.Fatal("无 name 不应命中")
	}
}

func TestScanAssignment(t *testing.T) {
	name, ok := scanAssignment(`ModName = "传送门模组"`, "ModName")
	if !ok || name != "传送门模组" {
		t.Fatalf("应得 传送门模组：%q ok=%v", name, ok)
	}
	// 词边界：MyModName 不应命中 ModName。
	if _, ok := scanAssignment(`MyModName = "x"`, "ModName"); ok {
		t.Fatal("词边界失败：MyModName 不应命中 ModName")
	}
}

func TestRustExt(t *testing.T) {
	cases := map[string]string{
		"foo.py":     "py",
		"foo.PY":     "PY",
		"a.b.json":   "json",
		"noext":      "",
		".hidden":    "",
		"trailing.":  "",
		"dir/x.lang": "lang",
	}
	for in, want := range cases {
		if got := rustExt(in); got != want {
			t.Fatalf("rustExt(%q)=%q 期望 %q", in, got, want)
		}
	}
}

func TestCountFiles_Classifies(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.py"), "x")
	mustWrite(t, filepath.Join(dir, "b.PY"), "x") // count_files 小写化扩展名 → 归 py（与区分大小写的 find_python_files 不同）
	must(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	mustWrite(t, filepath.Join(dir, "sub", "c.json"), "{}")
	mustWrite(t, filepath.Join(dir, "sub", "d.frag"), "x")     // 不在表 → other
	mustWrite(t, filepath.Join(dir, "sub", "e.fragment"), "x") // shader
	mustWrite(t, filepath.Join(dir, "noext"), "x")             // other

	counts := countFiles(dir)
	if counts["py"] != 2 {
		t.Fatalf("py 应 2（a.py + b.PY 小写化）：%v", counts)
	}
	if counts["json"] != 1 {
		t.Fatalf("json 应 1：%v", counts)
	}
	if counts["shader"] != 1 {
		t.Fatalf("shader 应 1（.fragment）：%v", counts)
	}
	if counts["other"] != 2 {
		t.Fatalf("other 应 2（d.frag/noext）：%v", counts)
	}
}

func TestBuild_FullMinimalFixture(t *testing.T) {
	dir := t.TempDir()
	demoRoot := filepath.Join(dir, "6-1DemoMod")
	proj := filepath.Join(demoRoot, "传送门Demo")
	beh := filepath.Join(proj, "传送门Behavior")
	res := filepath.Join(proj, "传送门Resource")
	must(t, os.MkdirAll(beh, 0o755))
	must(t, os.MkdirAll(res, 0o755))
	mustWrite(t, filepath.Join(beh, "modMain.py"), `
import mod.server.extraServerApi as serverApi
@Mod.Binding(name="PortalMod", version="0.0.1")
class PortalMod(object):
    @Mod.InitServer()
    def InitServer(self):
        serverApi.RegisterSystem("PortalMod", "PortalServer", "PortalMod.scripts.PortalServer.PortalServer")
`)
	mustWrite(t, filepath.Join(beh, "PortalServer.py"), `
class PortalServer(ServerSystem):
    def __init__(self, namespace, name):
        ServerSystem.__init__(self, namespace, name)
        self.compFactory = serverApi.GetEngineCompFactory()
        self.ListenForEvent(serverApi.GetEngineNamespace(), serverApi.GetEngineSystemName(), "AddServerPlayerEvent", self, self.OnPlayerAdd)
    def OnPlayerAdd(self, args):
        block = self.compFactory.CreateBlockInfo(args["playerId"])
        block.GetBlock((0,0,0))
`)

	outPath := filepath.Join(dir, "templates-index.json")
	report, err := buildTemplatesIndex(demoRoot, outPath)
	if err != nil {
		t.Fatalf("build 失败：%v", err)
	}
	if report.Total != 1 {
		t.Fatalf("total 应 1，得 %d", report.Total)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	txt := string(raw)
	for _, want := range []string{"PortalMod", "AddServerPlayerEvent", "BlockInfo", "传送门", "维度", "PortalServer"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("输出缺少 %q：\n%s", want, txt)
		}
	}

	// 结构化校验：snake_case key、systems 保 RegisterSystem 形状、apis 形状。
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("输出非合法 JSON 数组：%v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("应 1 条目，得 %d", len(entries))
	}
	e := entries[0]
	for _, key := range []string{"id", "path", "mod_name", "behavior_pack", "resource_pack", "tags", "systems", "events_used", "apis", "has_client", "has_ui", "has_custom_entity", "has_custom_items", "has_custom_blocks", "file_count"} {
		if _, ok := e[key]; !ok {
			t.Fatalf("缺少 snake_case key %q：%v", key, e)
		}
	}
	if got := string(e["mod_name"]); got != `"PortalMod"` {
		t.Fatalf("mod_name 应 PortalMod：%s", got)
	}
	if got := string(e["behavior_pack"]); got != `"传送门Behavior"` {
		t.Fatalf("behavior_pack 异常：%s", got)
	}
	if got := string(e["path"]); got != `"6-1DemoMod/传送门Demo"` {
		t.Fatalf("path 异常：%s", got)
	}
	// systems 应含一条 {cls_path,name,namespace}（serde_json 字典序 key）。
	var systems []map[string]string
	if err := json.Unmarshal(e["systems"], &systems); err != nil || len(systems) != 1 {
		t.Fatalf("systems 异常：%s err=%v", e["systems"], err)
	}
	if systems[0]["namespace"] != "PortalMod" || systems[0]["name"] != "PortalServer" {
		t.Fatalf("system 字段异常：%v", systems[0])
	}
}

func TestBuild_EmptyAndNonNilCollections(t *testing.T) {
	dir := t.TempDir()
	demoRoot := filepath.Join(dir, "demo")
	// 一个有 behavior 子目录但无任何 py / 标志文件的项目 → 空集合应渲染 [] / {components:[],methods:[]}。
	beh := filepath.Join(demoRoot, "EmptyProj", "fooBehavior")
	must(t, os.MkdirAll(beh, 0o755))

	outPath := filepath.Join(dir, "templates-index.json")
	if _, err := buildTemplatesIndex(demoRoot, outPath); err != nil {
		t.Fatalf("build 失败：%v", err)
	}
	raw, _ := os.ReadFile(outPath)
	txt := string(raw)
	// 空切片必须渲染成 [] 而非 null。
	if strings.Contains(txt, "null") {
		t.Fatalf("不应出现 null（空集合须 []/{}）：\n%s", txt)
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || len(entries) != 1 {
		t.Fatalf("解析失败或条目数异常：%v", err)
	}
	e := entries[0]
	if string(e["systems"]) != "[]" {
		t.Fatalf("空 systems 应 []：%s", e["systems"])
	}
	if string(e["events_used"]) != "[]" {
		t.Fatalf("空 events_used 应 []：%s", e["events_used"])
	}
	var tags []string
	if err := json.Unmarshal(e["tags"], &tags); err != nil || !reflect.DeepEqual(tags, []string{"EmptyProj"}) {
		t.Fatalf("无命中 tags 应回退 [项目名]：%s err=%v", e["tags"], err)
	}
	var apis map[string]json.RawMessage
	if err := json.Unmarshal(e["apis"], &apis); err != nil {
		t.Fatalf("apis 非对象：%s", e["apis"])
	}
	if string(apis["components"]) != "[]" || string(apis["methods"]) != "[]" {
		t.Fatalf("空 apis 应 {components:[],methods:[]}：%s", e["apis"])
	}
}

func TestBuild_NoProjectsEmptyArray(t *testing.T) {
	dir := t.TempDir()
	demoRoot := filepath.Join(dir, "demo")
	must(t, os.MkdirAll(demoRoot, 0o755))
	outPath := filepath.Join(dir, "out.json")
	report, err := buildTemplatesIndex(demoRoot, outPath)
	if err != nil {
		t.Fatalf("build 失败：%v", err)
	}
	if report.Total != 0 {
		t.Fatalf("无项目 total 应 0，得 %d", report.Total)
	}
	raw, _ := os.ReadFile(outPath)
	if string(raw) != "[]" {
		t.Fatalf("无项目应输出 []：%q", string(raw))
	}
}

func TestBuild_MissingDemoDirErrors(t *testing.T) {
	dir := t.TempDir()
	_, err := buildTemplatesIndex(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "out.json"))
	if err == nil {
		t.Fatal("demo 目录不存在应返错")
	}
}
