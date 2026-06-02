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

	"golang.org/x/text/encoding/simplifiedchinese"
)

// ── 纯函数单测（移植 http_sync.rs tests）────────────────────────────────

func TestExtractLinks_FindsRelativePaths(t *testing.T) {
	html := `
		<html><body>
		  <a href="api.html">api</a>
		  <a href='events.html'>events</a>
		  <a href="sub/dir.html">nested</a>
		</body></html>
	`
	got := extractLinks(html, "https://mc.163.com/dev/mcmanual/mc-dev/")
	want := []string{
		"https://mc.163.com/dev/mcmanual/mc-dev/api.html",
		"https://mc.163.com/dev/mcmanual/mc-dev/events.html",
		"https://mc.163.com/dev/mcmanual/mc-dev/sub/dir.html",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestExtractLinks_SkipsExternal(t *testing.T) {
	html := `
		<a href="https://google.com/foo.html">ext</a>
		<a href="https://mc.163.com/other/area.html">other-area</a>
		<a href="local.html">local</a>
	`
	got := extractLinks(html, "https://mc.163.com/dev/mcmanual/mc-dev/")
	for _, u := range got {
		if strings.Contains(u, "google.com") {
			t.Fatalf("外站不应保留: %v", got)
		}
	}
	if !containsStr(got, "https://mc.163.com/other/area.html") {
		t.Fatalf("同 host other-area 应保留: %v", got)
	}
	if !containsStr(got, "https://mc.163.com/dev/mcmanual/mc-dev/local.html") {
		t.Fatalf("相对 local 应保留: %v", got)
	}
}

func TestExtractLinks_SkipsAnchorsAndSchemes(t *testing.T) {
	html := `
		<a href="#top">anchor</a>
		<a href="javascript:void(0)">js</a>
		<a href="mailto:foo@bar">mail</a>
		<a href="page.html#section">with-fragment</a>
	`
	got := extractLinks(html, "https://mc.163.com/dev/mcmanual/mc-dev/")
	for _, u := range got {
		if strings.Contains(u, "javascript") || strings.Contains(u, "mailto") {
			t.Fatalf("锚点/scheme 不应保留: %v", got)
		}
	}
	if !containsStr(got, "https://mc.163.com/dev/mcmanual/mc-dev/page.html") {
		t.Fatalf("带 fragment 应剥后保留: %v", got)
	}
}

func TestURLToSubpathPlain(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://mc.163.com/dev/mcmanual/mc-dev/api.html", "dev/mcmanual/mc-dev/api.md"},
		{"https://mc.163.com/dev/mcmanual/mc-dev/sub/nested/page.htm", "dev/mcmanual/mc-dev/sub/nested/page.md"},
		{"https://mc.163.com/foo/bar/", "foo/bar/index.md"},
	}
	for _, c := range cases {
		if got := urlToSubpathPlain(c.in); got != c.want {
			t.Fatalf("urlToSubpathPlain(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestExtractAppJSURL(t *testing.T) {
	hrefHTML := `
		<html><head>
		  <link rel="preload" as="script" href="/dev/mcmanual/mc-dev/assets/js/app.abc123.js" />
		</head></html>
	`
	if got, ok := extractAppJSURL(hrefHTML); !ok || got != "/dev/mcmanual/mc-dev/assets/js/app.abc123.js" {
		t.Fatalf("href 形式: got=%q ok=%v", got, ok)
	}
	unquoted := "some inline JS referencing /dev/mcmanual/mc-dev/assets/js/app.deadbeef.js elsewhere"
	if got, ok := extractAppJSURL(unquoted); !ok || got != "/dev/mcmanual/mc-dev/assets/js/app.deadbeef.js" {
		t.Fatalf("裸引用形式: got=%q ok=%v", got, ok)
	}
	if _, ok := extractAppJSURL("<html><body>no bundle here</body></html>"); ok {
		t.Fatal("无 bundle 应返回 false")
	}
}

func TestPercentDecode(t *testing.T) {
	// UTF-8 中文
	if got, ok := percentDecode("/mcdocs/%E4%B8%AD%E6%96%87.html"); !ok || got != "/mcdocs/中文.html" {
		t.Fatalf("UTF-8 中文: got=%q ok=%v", got, ok)
	}
	// 纯 ASCII
	if got, ok := percentDecode("/mcdocs/plain.html"); !ok || got != "/mcdocs/plain.html" {
		t.Fatalf("ASCII: got=%q ok=%v", got, ok)
	}
	// GBK 回退
	chinese := "/mcdocs/接口/刚体.html"
	enc := gbkPercentEncode(t, chinese)
	if got, ok := percentDecode(enc); !ok || got != chinese {
		t.Fatalf("GBK 回退: got=%q ok=%v (enc=%s)", got, ok, enc)
	}
	// 双失败 → false
	if _, ok := percentDecode("/x/%FF%FE"); ok {
		t.Fatal("0xFF 0xFE 应两端解码都失败 → false")
	}
}

func gbkPercentEncode(t *testing.T, s string) string {
	t.Helper()
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatalf("GBK 编码: %v", err)
	}
	var sb strings.Builder
	for _, b := range gbk {
		if (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
			b == '/' || b == '.' || b == '-' || b == '_' {
			sb.WriteByte(b)
		} else {
			sb.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return sb.String()
}

func TestExtractVuepressRoutes(t *testing.T) {
	appJS := `
		... irrelevant code ...
		var __routes__ = [
		  { path:"/mcdocs/", name:"v-1" },
		  { path:"/mcdocs/api/event.html", name:"v-2" },
		  { path:"/mcdocs/%E4%B8%AD%E6%96%87.html", name:"v-3" },
		  { path:"/mcdocs/dup.html", name:"v-dup-1" },
		  { path:"/mcdocs/dup.html", name:"v-dup-2" }
		];
		... more bundle ...
	`
	routes := extractVuepressRoutes(appJS)
	for _, want := range []string{"/mcdocs/", "/mcdocs/api/event.html", "/mcdocs/中文.html", "/mcdocs/dup.html"} {
		if !containsStr(routes, want) {
			t.Fatalf("缺路由 %q: %v", want, routes)
		}
	}
	dup := 0
	for _, r := range routes {
		if r == "/mcdocs/dup.html" {
			dup++
		}
	}
	if dup != 1 {
		t.Fatalf("dup 应去重为 1，得 %d", dup)
	}
	// 排序：升序字节序
	for i := 1; i < len(routes); i++ {
		if routes[i-1] > routes[i] {
			t.Fatalf("未排序: %v", routes)
		}
	}
}

func TestAbsoluteURLFromRoute(t *testing.T) {
	base := "https://mc.163.com/dev/mcmanual/mc-dev/mcdocs/"
	if got := absoluteURLFromRoute(base, "/mcdocs/foo.html"); got != "https://mc.163.com/dev/mcmanual/mc-dev/mcdocs/foo.html" {
		t.Fatalf("mcdocs base: %q", got)
	}
	if got := absoluteURLFromRoute(base, "/mcdocs/sub/dir/page.html"); got != "https://mc.163.com/dev/mcmanual/mc-dev/mcdocs/sub/dir/page.html" {
		t.Fatalf("nested: %q", got)
	}
}

func TestVuepressRouteToSubpath(t *testing.T) {
	target := filepath.FromSlash("/tmp/data/docs/mcdocs")
	if got := vuepressRouteToSubpath("/mcdocs/foo/bar.html", target); got != "foo/bar.md" {
		t.Fatalf("strip leaf: %q", got)
	}
	if got := vuepressRouteToSubpath("/mcdocs/", target); got != "index.md" {
		t.Fatalf("root: %q", got)
	}
}

func TestExtractVuepressContent(t *testing.T) {
	html := `
		<html><body>
		  <div class="theme-container">
			<header>navbar</header>
			<main>
			  <h1 id="title">Hello <a href="#title" class="header-anchor">#</a></h1>
			  <p>body text</p>
			  <div class="page-info-hide">meta</div>
			  <footer>copyright</footer>
			</main>
		  </div>
		</body></html>
	`
	got, ok := extractVuepressContent(html)
	if !ok {
		t.Fatal("应抽取成功")
	}
	for _, want := range []string{"<h1", "Hello", "body text"} {
		if !strings.Contains(got, want) {
			t.Fatalf("应含 %q: %q", want, got)
		}
	}
	for _, no := range []string{"header-anchor", "page-info-hide", "copyright"} {
		if strings.Contains(got, no) {
			t.Fatalf("不应含 %q: %q", no, got)
		}
	}
	if _, ok := extractVuepressContent("<html><body><p>no headings here</p></body></html>"); ok {
		t.Fatal("无 <h1> 应返回 false")
	}
}

func TestStripHeaderAnchors(t *testing.T) {
	html := `<h2>Title <a href="#x" class="header-anchor">#</a></h2><p>keep <a href="/other">link</a></p>`
	got := stripHeaderAnchors(html)
	if strings.Contains(got, "header-anchor") {
		t.Fatalf("header-anchor 应删: %q", got)
	}
	if !strings.Contains(got, `<a href="/other">link</a>`) {
		t.Fatalf("普通 <a> 应留: %q", got)
	}
}

func TestTransformCustomBlocks(t *testing.T) {
	html := `<div class="custom-block tip"><p class="custom-block-title">提示</p><p>注意事项</p></div>`
	got := transformCustomBlocks(html)
	for _, want := range []string{"<blockquote>", "<strong>提示</strong>", "注意事项"} {
		if !strings.Contains(got, want) {
			t.Fatalf("应含 %q: %q", want, got)
		}
	}
	if strings.Contains(got, "custom-block") {
		t.Fatalf("不应留 custom-block: %q", got)
	}
	// unknown flavor 原样穿透
	unknown := `<div class="custom-block info"><p class="custom-block-title">x</p>y</div>`
	if got := transformCustomBlocks(unknown); !strings.Contains(got, `<div class="custom-block info">`) {
		t.Fatalf("info flavor opener 应原样: %q", got)
	}
}

func TestRewriteInternalLinks(t *testing.T) {
	md := "see [event](/dev/mcmanual/mc-dev/mcdocs/api/event.html) and " +
		"[guide](/dev/mcmanual/mc-dev/mcguide/foo.html) and " +
		"[online](/dev/mcmanual/mc-dev/mconline/bar.html#sec)"
	got := rewriteInternalLinks(md, "mcdocs")
	for _, want := range []string{"(./api/event.md)", "(../mcguide/foo.md)", "(../mconline/bar.md#sec)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("应含 %q: %q", want, got)
		}
	}
	if strings.Contains(got, ".html") {
		t.Fatalf("不应留 .html: %q", got)
	}
	// 无关链接不动
	ext := "[ext](https://example.com/foo.html)"
	if got := rewriteInternalLinks(ext, "mcdocs"); got != ext {
		t.Fatalf("外链应不动: %q", got)
	}
}

func TestFindMatchingDivClose(t *testing.T) {
	// body 从 opener 之后开始；嵌套一层 div。
	html := `<div class="custom-block tip">A<div>B</div>C</div>tail`
	start := strings.Index(html, ">") + 1 // opener 之后
	pos, ok := findMatchingDivClose(html, start)
	if !ok {
		t.Fatal("应找到匹配 </div>")
	}
	// pos 指向最外层 </div> 的 '<'
	if html[pos:pos+len("</div>")] != "</div>" {
		t.Fatalf("pos 未指向 </div>: %q", html[pos:])
	}
	if !strings.HasPrefix(html[pos+len("</div>"):], "tail") {
		t.Fatalf("应是最外层闭合: %q", html[pos:])
	}
}

func TestSplitBase(t *testing.T) {
	host, dir := splitBase("https://mc.163.com/dev/mcmanual/mc-dev/")
	if host != "https://mc.163.com" || dir != "/dev/mcmanual/mc-dev/" {
		t.Fatalf("host=%q dir=%q", host, dir)
	}
	host, dir = splitBase("https://mc.163.com/foo")
	if host != "https://mc.163.com" || dir != "/" {
		t.Fatalf("无目录: host=%q dir=%q", host, dir)
	}
}

// ── htmlToMarkdown converter 配置回归（spike 实测固化，R6）────────────────

func TestHTMLToMarkdownConfig(t *testing.T) {
	// 表格须转成管道表（默认 commonmark 会丢表，需 table 插件）。
	tableHTML := `<table><thead><tr><th>参数名</th><th>类型</th><th>描述</th></tr></thead><tbody><tr><td>pos</td><td>tuple</td><td>坐标</td></tr></tbody></table>`
	md, err := htmlToMarkdown(tableHTML)
	if err != nil {
		t.Fatalf("convert table: %v", err)
	}
	if !strings.Contains(md, "| 参数名") || !strings.Contains(md, "---") || !strings.Contains(md, "| pos") {
		t.Fatalf("表格应转管道表: %q", md)
	}
	// 示例 [code]...[/code] 须字面保留（EscapeModeDisabled，否则被转义成 \[code]）。
	codeHTML := `<p>示例 [code]player.setPos(1,2,3)[/code] 结束</p>`
	md, err = htmlToMarkdown(codeHTML)
	if err != nil {
		t.Fatalf("convert code: %v", err)
	}
	if !strings.Contains(md, "[code]") || !strings.Contains(md, "[/code]") {
		t.Fatalf("[code] 应字面保留: %q", md)
	}
}

func TestDecodeBody(t *testing.T) {
	// 声明 GBK → GBK 解码。
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("接口文档"))
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeBody("text/html; charset=gbk", gbk); got != "接口文档" {
		t.Fatalf("GBK body: %q", got)
	}
	// 默认 UTF-8。
	if got := decodeBody("text/html; charset=utf-8", []byte("接口文档")); got != "接口文档" {
		t.Fatalf("UTF-8 body: %q", got)
	}
	// 无 charset → 当 UTF-8 直通。
	if got := decodeBody("text/html", []byte("hello")); got != "hello" {
		t.Fatalf("默认直通: %q", got)
	}
}

// ── 端到端 httptest（移植 sync_mock_server_plain / _vuepress_end_to_end）──

func TestHTTPSync_Plain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><body>
				<a href="page1.html">p1</a>
				<a href="page2.html">p2</a>
				<a href="#anchor">skip</a>
				<a href="javascript:void(0)">skip2</a>
				</body></html>`)
		case "/page1.html":
			fmt.Fprint(w, "<html><body><h1>Page One</h1><p>hello world</p></body></html>")
		case "/page2.html":
			fmt.Fprint(w, "<html><body><h1>Page Two</h1><p>second page</p></body></html>")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	report, err := httpSync(context.Background(), &http.Client{}, HTTPSyncParams{
		SourceName:  "mock",
		BaseURL:     srv.URL + "/",
		TargetDir:   tmp,
		Concurrency: 2,
		Mode:        HTTPSyncModePlain,
	}, NewProgressSink(nil))
	if err != nil {
		t.Fatalf("httpSync: %v", err)
	}
	if report.Fetched != 2 {
		t.Fatalf("fetched=%d want 2", report.Fetched)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("failed: %+v", report.Failed)
	}
	p1, _ := os.ReadFile(filepath.Join(tmp, "page1.md"))
	if !strings.Contains(string(p1), "Page One") && !strings.Contains(string(p1), "hello") {
		t.Fatalf("page1.md 内容异常: %q", p1)
	}
	if _, err := os.Stat(filepath.Join(tmp, "page2.md")); err != nil {
		t.Fatalf("page2.md 缺失: %v", err)
	}
}

func TestHTTPSync_VuepressEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/dev/mcmanual/mc-dev/mcdocs/":
			fmt.Fprint(w, `<html><head>
				<link rel="preload" as="script" href="/dev/mcmanual/mc-dev/assets/js/app.abcdef.js" />
				</head><body><div id="app"></div></body></html>`)
		case "/dev/mcmanual/mc-dev/assets/js/app.abcdef.js":
			fmt.Fprint(w, `var routes = [
				{ path:"/mcdocs/" },
				{ path:"/mcdocs/api/event.html" },
				{ path:"/mcdocs/guide/intro.html" }
				];`)
		case "/dev/mcmanual/mc-dev/mcdocs/api/event.html":
			fmt.Fprint(w, `<html><body><h1>Event API <a href="#event-api" class="header-anchor">#</a></h1><p>Event docs body</p><div class="page-nav">next</div></body></html>`)
		case "/dev/mcmanual/mc-dev/mcdocs/guide/intro.html":
			fmt.Fprint(w, `<html><body><h1>Intro</h1><p>Welcome</p><div class="custom-block tip"><p class="custom-block-title">Tip</p><p>be kind</p></div><div class="edit-link">edit</div></body></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "mcdocs") // 末段 mcdocs，使 route 前缀被剥
	report, err := httpSync(context.Background(), &http.Client{}, HTTPSyncParams{
		SourceName:  "mcdocs",
		BaseURL:     srv.URL + "/dev/mcmanual/mc-dev/mcdocs/",
		TargetDir:   targetDir,
		Concurrency: 2,
		Mode:        HTTPSyncModeVuepress,
	}, NewProgressSink(nil))
	if err != nil {
		t.Fatalf("httpSync vuepress: %v", err)
	}
	if report.Fetched != 2 {
		t.Fatalf("fetched=%d want 2 (failed=%+v)", report.Fetched, report.Failed)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("failed: %+v", report.Failed)
	}
	b1, err := os.ReadFile(filepath.Join(targetDir, "api", "event.md"))
	if err != nil {
		t.Fatalf("api/event.md 缺失: %v", err)
	}
	if !strings.Contains(string(b1), "Event API") {
		t.Fatalf("event.md 缺标题: %q", b1)
	}
	if strings.Contains(string(b1), "header-anchor") {
		t.Fatalf("anchor 未剥: %q", b1)
	}
	b2, err := os.ReadFile(filepath.Join(targetDir, "guide", "intro.md"))
	if err != nil {
		t.Fatalf("guide/intro.md 缺失: %v", err)
	}
	if !strings.Contains(string(b2), "Intro") {
		t.Fatalf("intro.md 缺标题: %q", b2)
	}
	if !strings.Contains(string(b2), "Tip") && !strings.Contains(string(b2), ">") {
		t.Fatalf("intro.md 缺 blockquote: %q", b2)
	}
}

func TestHTTPSync_VuepressNoH1Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/dev/mcmanual/mc-dev/mcdocs/":
			fmt.Fprint(w, `<html><head><link href="/dev/mcmanual/mc-dev/assets/js/app.abcdef.js" /></head></html>`)
		case "/dev/mcmanual/mc-dev/assets/js/app.abcdef.js":
			fmt.Fprint(w, `var routes = [{ path:"/mcdocs/noh1.html" }];`)
		case "/dev/mcmanual/mc-dev/mcdocs/noh1.html":
			fmt.Fprint(w, `<html><body><p>no heading</p></body></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	report, err := httpSync(context.Background(), &http.Client{}, HTTPSyncParams{
		SourceName:  "mcdocs",
		BaseURL:     srv.URL + "/dev/mcmanual/mc-dev/mcdocs/",
		TargetDir:   filepath.Join(tmp, "mcdocs"),
		Concurrency: 1,
		Mode:        HTTPSyncModeVuepress,
	}, NewProgressSink(nil))
	if err != nil {
		t.Fatalf("httpSync: %v", err)
	}
	if report.Fetched != 0 {
		t.Fatalf("fetched=%d want 0", report.Fetched)
	}
	if len(report.Failed) != 1 || !strings.Contains(report.Failed[0].Error, "no <h1>") {
		t.Fatalf("应有 1 条 no <h1> 失败: %+v", report.Failed)
	}
}
