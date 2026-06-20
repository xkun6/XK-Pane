package mcpe

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, raw string) ParsedLog {
	t.Helper()
	p, ok := parseStdoutLine(raw)
	if !ok {
		t.Fatalf("应保留: %q", raw)
	}
	return p
}

func mustDrop(t *testing.T, raw string) {
	t.Helper()
	if _, ok := parseStdoutLine(raw); ok {
		t.Fatalf("应丢弃: %q", raw)
	}
}

func TestStdoutStripsPythonPrefix(t *testing.T) {
	r := mustParse(t, "[Python] [2026-05-14 12:00:00,000] [INFO] [Server] hello")
	if r.Severity != sevInfo {
		t.Errorf("severity=%q", r.Severity)
	}
	if !strings.Contains(r.Display, "hello") || strings.HasPrefix(r.Display, "[Python]") {
		t.Errorf("display=%q", r.Display)
	}
}

func TestStdoutDropsCppEngineStructured(t *testing.T) {
	for _, s := range []string{
		"[2026-05-14 20:22:16:253 INFO PERF 56104 44944] SerialWorkList [OnAppStart] initPacks: calls 1",
		"[2026-05-14 20:22:17:436 WARN NETEASE 38104 44944] register_event_id BusID: 0",
		"[2026-05-14 20:22:20:386 ERROR ALL 38104 44944] ServerPlayer not found",
	} {
		mustDrop(t, s)
	}
}

func TestStdoutDropsCppEngineNoise(t *testing.T) {
	for _, s := range []string{
		"construct shareables definition",
		"Current world has 117 block permutations",
		"OpenGL highp vertex shader precision : 23",
		"Priorities:",
		"{ { DirectoryWatcher.cpp, 10920362103149636706, 198 } }",
	} {
		mustDrop(t, s)
	}
}

func TestStdoutDropsUnisdkAndCurlDebug(t *testing.T) {
	for _, s := range []string{
		"[UniSDK]setDebugMode = true",
		"*   Trying 45.253.145.166:443...",
		"> POST /config HTTP/2",
		"< HTTP/2 200 ",
		"Host: g79.appdump.nie.netease.com",
		"NO LOG FILE! - initEnvSDK!!!!!!!!!",
		`[python] Use mcp at "data/vanilla.mcp"`, // 小写 python
		`['G:\\MCStudioDownload', './lib']`,      // Python 列表 print
	} {
		mustDrop(t, s)
	}
}

func TestStdoutKeepsPythonStructuredDeveloperLog(t *testing.T) {
	r := mustParse(t, "[2026-05-14 20:22:15,891] [INFO][Developer] Starting mobile logger")
	if r.Timestamp != "2026-05-14 20:22:15,891" {
		t.Errorf("ts=%q", r.Timestamp)
	}
	if r.Level != "INFO" || r.Category != "Developer" {
		t.Errorf("level=%q cat=%q", r.Level, r.Category)
	}
	if r.Severity != sevDeveloper {
		t.Errorf("severity=%q", r.Severity)
	}
}

func TestStdoutKeepsPythonStructuredNoCategory(t *testing.T) {
	r := mustParse(t, "[2026-05-14 20:22:17,509] [INFO] sendWinPlayWorldDCLog: engine_launch gameserver_connect 0 start")
	if r.Level != "INFO" || r.Category != "" {
		t.Errorf("level=%q cat=%q", r.Level, r.Category)
	}
	if !strings.Contains(r.Display, "sendWinPlayWorldDCLog") {
		t.Errorf("display=%q", r.Display)
	}
}

func TestStdoutDropsInfoEngineKeepsErrorEngine(t *testing.T) {
	mustDrop(t, "[Python] [2026-05-14 21:30:22,493] [INFO][Engine] RegisterEngineHandler: x")
	mustDrop(t, "[2026-05-14 21:30:22,496] [INFO][Engine] register_event_id x")
	mustParse(t, "[Python] [2026-05-14 20:22:19,549] [ERROR][Engine] get_world_record: None")
	mustParse(t, "[2026-05-14 20:22:22,510] [WARN][Engine] degraded resource")
}

func TestStdoutKeepsPythonErrorEngine(t *testing.T) {
	r := mustParse(t, "[Python] [2026-05-14 20:22:19,549] [ERROR][Engine] get_world_record: None")
	if r.Severity != sevError || r.Level != "ERROR" || r.Category != "Engine" {
		t.Errorf("sev=%q level=%q cat=%q", r.Severity, r.Level, r.Category)
	}
	if strings.HasPrefix(r.Display, "[Python]") {
		t.Errorf("display should strip [Python]: %q", r.Display)
	}
}

func TestStdoutDropsEngineEmptyErrorDumpNoise(t *testing.T) {
	// post_error_dump 每帧空转储噪音（带 / 不带 [Python] 前缀都应丢弃）
	for _, s := range []string{
		"[2026-06-20 09:02:18,049] [ERROR][Engine] post_error_dump: Starting error dump",
		"[2026-06-20 09:02:18,052] [ERROR][Engine] Error dump: None",
		"[Python] [2026-06-20 09:02:18,049] [ERROR][Engine] post_error_dump: Starting error dump",
		"[Python] [2026-06-20 09:02:18,052] [ERROR][Engine] Error dump: None",
	} {
		mustDrop(t, s)
	}
}

func TestStdoutKeepsRealEngineErrorDump(t *testing.T) {
	// 只屏蔽 None：真有内容的 dump 必须保留，避免吞掉真实错误
	mustParse(t, "[2026-06-20 09:02:18,052] [ERROR][Engine] Error dump: Traceback (most recent call last)")
	// 其它 [ERROR][Engine]（如 get_world_record: None）不在屏蔽名单内，照常保留
	mustParse(t, "[2026-05-14 20:22:19,549] [ERROR][Engine] get_world_record: None")
}

func TestStdoutKeepsModBracketTag(t *testing.T) {
	cases := []struct{ input, cat string }{
		{"[ContainerTreasure] ContainerTreasure Server Init", "ContainerTreasure"},
		{"[Bakeshi] Register server system: x", "Bakeshi"},
		{"[ContainerTreasure][WARN] [ZiplineBoot] server __init__ entered", "ContainerTreasure"},
	}
	for _, c := range cases {
		r := mustParse(t, c.input)
		if r.Timestamp != "" || r.Level != "" {
			t.Errorf("%q: ts=%q level=%q should be empty", c.input, r.Timestamp, r.Level)
		}
		if r.Category != c.cat {
			t.Errorf("%q: cat=%q want %q", c.input, r.Category, c.cat)
		}
	}
}

func TestStdoutKeepsLoadWindowsAddonAndEcs(t *testing.T) {
	for _, s := range []string{
		"LoadWindowsAddonPy: CTScripts.modMain",
		"ecs_framework create entity VipLogicServerEntity -8589934591",
		"ecs_framework clear all",
	} {
		r := mustParse(t, s)
		if r.Timestamp != "" || r.Level != "" || r.Category != "" {
			t.Errorf("%q: should have no ts/level/cat", s)
		}
		if r.Display != s {
			t.Errorf("display=%q want %q", r.Display, s)
		}
	}
}

func TestStdoutStripsPythonPrefixKeepsRest(t *testing.T) {
	r := mustParse(t, "[Python] [Bakeshi] Notify main pack mapId=sanjiaozhou_map3")
	if strings.HasPrefix(r.Display, "[Python]") || !strings.HasPrefix(r.Display, "[Bakeshi]") {
		t.Errorf("display=%q", r.Display)
	}
}

func TestStdoutEmptyReturnsFalse(t *testing.T) {
	mustDrop(t, "")
	mustDrop(t, "\r\n")
}

func TestSeverityDeveloperOverridesInfo(t *testing.T) {
	r := mustParse(t, "[Python] [2026] [INFO] [Developer] print stuff")
	if r.Severity != sevDeveloper {
		t.Errorf("severity=%q want developer", r.Severity)
	}
}

func TestSeverityRecognizesLowercaseContentLogTags(t *testing.T) {
	if detectSeverity("[Scripting][error]-x") != sevError {
		t.Error("error")
	}
	if detectSeverity("[Texture][warning]-x") != sevWarn {
		t.Error("warn")
	}
	if detectSeverity("[Texture][info]-x") != sevInfo {
		t.Error("info")
	}
}
