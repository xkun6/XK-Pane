package bedrockdocsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWalkMdFiles_RecursesAndSkips(t *testing.T) {
	dir := t.TempDir()
	// root/a.md, root/sub/b.md, root/sub/索引.md(skip), root/sub/c.txt(skip)
	mustWrite(t, filepath.Join(dir, "a.md"), "# a")
	must(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	mustWrite(t, filepath.Join(dir, "sub", "b.md"), "# b")
	mustWrite(t, filepath.Join(dir, "sub", "索引.md"), "# index")
	mustWrite(t, filepath.Join(dir, "sub", "c.txt"), "x")

	files := walkMdFiles(dir, skipFilesAPI)
	if len(files) != 2 {
		t.Fatalf("应 2 个 md，得 %v", files)
	}
	var names []string
	for _, p := range files {
		names = append(names, filepath.Base(p))
	}
	if !containsStr(names, "a.md") || !containsStr(names, "b.md") {
		t.Fatalf("应含 a.md/b.md：%v", names)
	}
}

func TestSplitH2Sections(t *testing.T) {
	md := "preamble\n\n## Foo\nfoo body line 1\nfoo body line 2\n## Bar\nbar body\n"
	secs := splitH2Sections(md)
	if len(secs) != 2 {
		t.Fatalf("应 2 段，得 %d", len(secs))
	}
	if secs[0][0] != "Foo" || !strings.Contains(secs[0][1], "foo body line 1") {
		t.Fatalf("Foo 段异常: %+v", secs[0])
	}
	if secs[1][0] != "Bar" || !strings.Contains(secs[1][1], "bar body") {
		t.Fatalf("Bar 段异常: %+v", secs[1])
	}
}

func TestParseParamsTable(t *testing.T) {
	lines := []string{"参数名 | 类型 | 描述", "--- | --- | ---", "playerId | str | 玩家ID", "amount | int | 数量"}
	params := parseParamsTable(lines)
	if len(params) != 2 {
		t.Fatalf("应 2 行，得 %d", len(params))
	}
	if params[0].Name != "playerId" || params[0].Ty != "str" || params[0].Description != "玩家ID" {
		t.Fatalf("首行异常: %+v", params[0])
	}
	// 两列（省略描述列）
	two := parseParamsTable([]string{"--- | ---", "x | int", "y | str"})
	if len(two) != 2 || two[1].Name != "y" || two[1].Ty != "str" || two[1].Description != "" {
		t.Fatalf("两列异常: %+v", two)
	}
}

func TestExtractCodeBlock(t *testing.T) {
	text := "preamble\n[code]\ndef foo():\n    pass\n[/code]\nepilogue"
	ex := extractCodeBlock(text)
	if !strings.Contains(ex, "def foo()") || strings.Contains(ex, "preamble") {
		t.Fatalf("提取异常: %q", ex)
	}
	if extractCodeBlock("no code here") != "" {
		t.Fatal("无配对应返空")
	}
	if extractCodeBlock("[CODE]x[/CODE]") != "" {
		t.Fatal("大写不应命中")
	}
}

func TestParseSections_SingleSideServer(t *testing.T) {
	md := "## MyApi\n\n服务端\n\n  * 描述\n    Test description line.\n"
	entries := parseSections(md, "api", "/docs/mcdocs/test.md")
	if len(entries) != 1 {
		t.Fatalf("应 1 条，得 %d", len(entries))
	}
	e := entries[0]
	if e.Kind != "api" || e.Name != "MyApi" || e.Side != "server" {
		t.Fatalf("条目异常: %+v", e)
	}
	if !strings.Contains(e.Description, "Test description") {
		t.Fatalf("描述异常: %q", e.Description)
	}
}

func TestNormalizeSetext_ConvertsToATX(t *testing.T) {
	md := " 地图\n==========\n\n CanSee\n----------\n\nbody line\n"
	got := normalizeSetextHeadings(md)
	if !strings.Contains(got, "# 地图\n") || !strings.Contains(got, "## CanSee\n") {
		t.Fatalf("setext→atx 异常: %q", got)
	}
	if strings.Contains(got, "==========") || strings.Contains(got, "----------") {
		t.Fatalf("下划线行应消失: %q", got)
	}
	if !strings.Contains(got, "body line") {
		t.Fatalf("正文应保留: %q", got)
	}
}

func TestNormalizeSetext_LeavesTableSeparatorsAlone(t *testing.T) {
	md := "name | type\n---|---\nfoo | int\n"
	got := normalizeSetextHeadings(md)
	if !strings.Contains(got, "name | type") || !strings.Contains(got, "---|---") {
		t.Fatalf("表分隔行不应被改: %q", got)
	}
	if strings.Contains(got, "# name") || strings.Contains(got, "## name") {
		t.Fatalf("name 不应变标题: %q", got)
	}
}

func TestSplitSubParts_AcceptsUnindentedBullets(t *testing.T) {
	body := "* 描述\n\nfirst sentence\nsecond sentence\n\n* 参数\n\nparams body\n"
	parts := splitSubParts(body)
	if len(parts) != 2 {
		t.Fatalf("应 2 段，得 %d", len(parts))
	}
	if !strings.HasPrefix(parts[0], "描述") || !strings.Contains(parts[0], "first sentence") {
		t.Fatalf("描述段异常: %q", parts[0])
	}
	if !strings.HasPrefix(parts[1], "参数") || !strings.Contains(parts[1], "params body") {
		t.Fatalf("参数段异常: %q", parts[1])
	}
}

func TestSplitSubParts_KeepsNestedBulletsInBody(t *testing.T) {
	body := "* 备注\n\n    * angleX 解释\n    * angleY 解释\n\n* 示例\n\n[code]\nfoo\n[/code]\n"
	parts := splitSubParts(body)
	if len(parts) != 2 {
		t.Fatalf("嵌套 bullet 不应起新 sub-part，应 2 段得 %d: %q", len(parts), parts)
	}
	if !strings.HasPrefix(parts[0], "备注") || !strings.Contains(parts[0], "angleX 解释") {
		t.Fatalf("备注段异常: %q", parts[0])
	}
	if !strings.HasPrefix(parts[1], "示例") {
		t.Fatalf("示例段异常: %q", parts[1])
	}
}

func TestSplitH3Sections_TrimsTrailingHashes(t *testing.T) {
	body := "###  服务端接口 ###\n\nserver body\n\n###  客户端接口 ###\n\nclient body\n"
	secs := splitH3Sections(body)
	if len(secs) != 2 {
		t.Fatalf("应 2 段，得 %d", len(secs))
	}
	if secs[0][0] != "服务端接口" || secs[1][0] != "客户端接口" {
		t.Fatalf("尾随 # 未去净: %q / %q", secs[0][0], secs[1][0])
	}
}

func TestParseSections_HandlesHTML2MDSetextFormat(t *testing.T) {
	md := " 地图\n==========\n\n" +
		" CanSee\n----------\n\n" +
		"服务端客户端\n\n" +
		"###  服务端接口 ###\n\n" +
		"method in mod.server.component.gameCompServer.GameComponentServer\n\n" +
		"* 描述\n\n判断起始对象是否可看见目标对象\n\n" +
		"* 参数\n\n参数名 | 数据类型 | 说明\n---|---|---\n" +
		"fromId | str | 起始对象ID\n\n" +
		"* 返回值\n\n数据类型 | 说明\n---|---\nbool | 是否可见\n\n" +
		"* 备注\n\n    * angleX 解释\n\n" +
		"* 示例\n\n[code]\nfoo()\n[/code]\n\n" +
		"###  客户端接口 ###\n\n" +
		"method in mod.client.component.gameCompClient.GameComponentClient\n\n" +
		"* 描述\n\nclient side描述\n"
	normalized := normalizeSetextHeadings(md)
	entries := parseSections(normalized, "api", "/docs/mcdocs/test.md")
	if len(entries) != 2 {
		t.Fatalf("应每 side 一条共 2 条，得 %d", len(entries))
	}
	for _, e := range entries {
		if e.Name != "CanSee" {
			t.Fatalf("name 应 CanSee: %q", e.Name)
		}
	}
	var server *OutputAPIEntry
	sides := map[string]bool{}
	for i := range entries {
		sides[entries[i].Side] = true
		if entries[i].Side == "server" {
			server = &entries[i]
		}
	}
	if !sides["server"] || !sides["client"] {
		t.Fatalf("应含 server+client: %v", sides)
	}
	if server == nil {
		t.Fatal("缺 server 条目")
	}
	if !strings.Contains(server.Description, "判断起始对象") {
		t.Fatalf("server 描述异常: %q", server.Description)
	}
	if len(server.Params) != 1 || server.Params[0].Name != "fromId" || server.Params[0].Ty != "str" {
		t.Fatalf("server 参数异常: %+v", server.Params)
	}
	if len(server.Returns) != 1 || server.Returns[0].Ty != "bool" {
		t.Fatalf("server 返回值异常: %+v", server.Returns)
	}
	if !strings.Contains(server.Remark, "angleX 解释") {
		t.Fatalf("server 备注异常: %q", server.Remark)
	}
	if !strings.Contains(server.Example, "foo()") {
		t.Fatalf("server 示例异常: %q", server.Example)
	}
}

func TestBuildAPIIndex_FullMinimalFixture(t *testing.T) {
	dir := t.TempDir()
	mcdocsRoot := filepath.Join(dir, "mcdocs")
	apiDir := filepath.Join(mcdocsRoot, "1-ModAPI", "接口", "sub")
	eventDir := filepath.Join(mcdocsRoot, "1-ModAPI", "事件")
	apiIdxDir := filepath.Join(mcdocsRoot, "1-ModAPI", "接口")
	must(t, os.MkdirAll(apiDir, 0o755))
	must(t, os.MkdirAll(eventDir, 0o755))
	mustWrite(t, filepath.Join(apiDir, "MyApi.md"), "## DoThing\n服务端\n\n  * 描述\n    Doing stuff.\n")
	mustWrite(t, filepath.Join(eventDir, "MyEvent.md"), "## ServerChunkLoadEvent\n服务端\n\n  * 描述\n    A chunk got loaded.\n")
	mustWrite(t, filepath.Join(apiIdxDir, "索引.md"), "# Index page that must be skipped")

	outPath := filepath.Join(dir, "data", "api-index.json")
	report, err := buildAPIIndex(mcdocsRoot, outPath)
	if err != nil {
		t.Fatalf("buildAPIIndex: %v", err)
	}
	if report.HeaderCount != 3 || report.APICount != 1 || report.EventCount != 1 || report.Total != 5 {
		t.Fatalf("报告异常: %+v", report)
	}
	written, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("读输出: %v", err)
	}
	s := string(written)
	for _, want := range []string{"DoThing", "ServerChunkLoadEvent", `"type": "api"`, `"type": "event"`, `"type": "header"`, `"params": []`} {
		if !strings.Contains(s, want) {
			t.Fatalf("输出缺 %q:\n%s", want, s)
		}
	}
	// 不转义 HTML：确认无 < 之类（本 fixture 无 <>&，作回归保护）。
	if strings.Contains(s, `<`) || strings.Contains(s, `&`) {
		t.Fatalf("不应转义 HTML 字符: %s", s)
	}
	// 末尾无多余换行。
	if strings.HasSuffix(s, "\n") {
		t.Fatalf("输出末尾不应有换行")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
