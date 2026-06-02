package bedrockdocsync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// http_sync.go：基于 HTTP 抓取的源同步（对齐 http_sync.rs）。两种爬取模式：
//   - Plain：GET 索引页，提取同源 <a href>，逐页抓取转 Markdown。
//   - Vuepress：针对 mc.163.com mcdocs 的 VuePress 1.x SPA，读 app.<hash>.js 还原
//     路由表，抓每个叶子页并切出正文（<h1> ... page-info-hide/page-nav/edit-link/footer）
//     再转 Markdown。两模式共用抓取/写盘管线，分叉只在 URL 发现 + HTML 预处理。
//
// 保真原则：除 HTML→Markdown 核心外，所有发现/切片/清洗/链接重写都用手写字节扫描逐字
// 复刻 Rust 原文（无正则、无 DOM 解析）。Rust 的 str.find()/&s[a..b] 都是字节偏移；Go 的
// string + strings.Index 同为字节偏移，二者天然一致，全程不转 []rune。

// HTTPSyncMode 是 httpSync 的爬取策略（对齐 HttpSyncMode）。
type HTTPSyncMode int

const (
	// HTTPSyncModePlain 扫索引页 <a href> 逐个抓。
	HTTPSyncModePlain HTTPSyncMode = iota
	// HTTPSyncModeVuepress 走 app.<hash>.js 路由表 + 文章体切片。
	HTTPSyncModeVuepress
)

// HTTPSyncParams 一次 HTTP 同步入参（纯内部参数，无 JSON tag，对齐 GiteaSyncParams）。
type HTTPSyncParams struct {
	SourceName  string       // 进度事件里的稳定标识（如 "mcdocs"）
	BaseURL     string       // 索引页 URL，兼作链接解析 base，建议带尾斜杠
	TargetDir   string       // 输出目录（缺失则建），文件镜像 URL 路径并 .html→.md
	Concurrency int          // 并发抓取页数；调用方负责 clamp
	Mode        HTTPSyncMode // 发现 + 抽取策略
}

// HTTPSyncReport 一次 HTTP 同步报告（对齐 HttpSyncReport：camelCase JSON tag）。
type HTTPSyncReport struct {
	SourceName string         `json:"sourceName"`
	Fetched    uint32         `json:"fetched"` // 成功写盘页数 → 写进 SourceInfo.FileCount
	Failed     []FetchFailure `json:"failed"`  // 复用 gitea.go 的 FetchFailure{Path,Error}
}

// crawlTarget 是一个待抓页：绝对 URL + 相对 TargetDir 的落盘路径（已 .md，始终正斜杠）。
type crawlTarget struct {
	url           string
	outputSubpath string
}

// httpSync 执行一次 HTTP/VuePress 同步。client 由调用方按 ProxyConfig 构造（便于测试注入）。
func httpSync(ctx context.Context, client *http.Client, params HTTPSyncParams, progress ProgressSink) (HTTPSyncReport, error) {
	if err := os.MkdirAll(params.TargetDir, 0o755); err != nil {
		return HTTPSyncReport{}, fmt.Errorf("mkdir target: %w", err)
	}

	// 1. 发现待抓 URL。
	progress.Info(fmt.Sprintf("%s: discovering routes from %s", params.SourceName, params.BaseURL))
	var targets []crawlTarget
	switch params.Mode {
	case HTTPSyncModePlain:
		indexHTML, err := fetchText(ctx, client, params.BaseURL)
		if err != nil {
			return HTTPSyncReport{}, fmt.Errorf("fetch index: %w", err)
		}
		for _, url := range extractLinks(indexHTML, params.BaseURL) {
			targets = append(targets, crawlTarget{url: url, outputSubpath: urlToSubpathPlain(url)})
		}
	case HTTPSyncModeVuepress:
		routes, err := discoverVuepressRoutes(ctx, client, params.BaseURL)
		if err != nil {
			return HTTPSyncReport{}, fmt.Errorf("discover vuepress routes: %w", err)
		}
		for _, pair := range routes {
			targets = append(targets, crawlTarget{
				url:           pair[0],
				outputSubpath: vuepressRouteToSubpath(pair[1], params.TargetDir),
			})
		}
	}

	total := uint32(len(targets))
	progress.Info(fmt.Sprintf("%s: discovered %d pages", params.SourceName, total))

	// 2. 并发抓取 + 转换（worker pool + 信号量，照搬 gitea.go 结构；部分失败不致命）。
	concurrency := params.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var fetched uint32
	var failed []FetchFailure

	recordFail := func(url, msg string) {
		mu.Lock()
		failed = append(failed, FetchFailure{Path: url, Error: msg})
		mu.Unlock()
	}

	for _, t := range targets {
		if ctx.Err() != nil {
			break // 取消：停止派发新任务
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(t crawlTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			md, err := fetchAndConvert(ctx, client, t.url, params.Mode, params.SourceName)
			if err != nil {
				recordFail(t.url, err.Error())
				return
			}
			dest := filepath.Join(params.TargetDir, filepath.FromSlash(t.outputSubpath))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				recordFail(t.url, fmt.Sprintf("mkdir parent: %v", err))
				return
			}
			if err := os.WriteFile(dest, []byte(md), 0o644); err != nil {
				recordFail(t.url, fmt.Sprintf("write md file: %v", err))
				return
			}
			mu.Lock()
			fetched++
			f := fetched
			mu.Unlock()
			if f%10 == 0 {
				progress.HTTPFetch(params.SourceName, f, total)
			}
		}(t)
	}
	wg.Wait()

	// 末尾必发一次，让 UI 总能看到 fetched≈total 的事件。
	progress.HTTPFetch(params.SourceName, fetched, total)

	return HTTPSyncReport{
		SourceName: params.SourceName,
		Fetched:    fetched,
		Failed:     failed,
	}, nil
}

func fetchAndConvert(ctx context.Context, client *http.Client, url string, mode HTTPSyncMode, sourceName string) (string, error) {
	htmlStr, err := fetchText(ctx, client, url)
	if err != nil {
		return "", err
	}
	switch mode {
	case HTTPSyncModePlain:
		return htmlToMarkdown(htmlStr)
	case HTTPSyncModeVuepress:
		extracted, ok := extractVuepressContent(htmlStr)
		if !ok {
			return "", fmt.Errorf("no <h1> in %s", url)
		}
		md, err := htmlToMarkdown(extracted)
		if err != nil {
			return "", err
		}
		return rewriteInternalLinks(md, sourceName), nil
	}
	return "", fmt.Errorf("unknown mode")
}

// fetchText 抓取一个 URL 的文本正文（合并 send/status/body 三阶段，错误前缀对齐 Rust
// fetch_and_convert）。body 解码对齐 reqwest .text()：按 Content-Type charset，默认 UTF-8（R1）。
func fetchText(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("send: %w", err)
	}
	req.Header.Set("User-Agent", probeUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("body: %w", err)
	}
	return decodeBody(resp.Header.Get("Content-Type"), b), nil
}

// decodeBody 按 Content-Type charset 解码（对齐 reqwest .text() 语义）：声明 gbk/gb2312/gb18030
// 则 GBK 解码（无替换字符才接受），否则按 UTF-8 直通（非法序列不报错，等价 reqwest 的 lossy text）。
func decodeBody(contentType string, b []byte) string {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "charset=gbk") || strings.Contains(ct, "charset=gb2312") || strings.Contains(ct, "charset=gb18030") {
		if dec, err := simplifiedchinese.GBK.NewDecoder().Bytes(b); err == nil && !bytes.ContainsRune(dec, utf8.RuneError) {
			return string(dec)
		}
	}
	return string(b)
}

// htmlToMarkdown 用 html-to-markdown/v2 转换（替代 Rust html2md::parse_html）。每次新建 converter
// 以并发安全（与库自身的顶层 ConvertString 同模式）。配置经 spike 实测确定（见 http_sync_test.go）：
//   - table 插件：默认 commonmark 会丢 <table>，加它才产管道表（参数表/返回值表需要）。
//   - EscapeModeDisabled：默认 smart 会把示例里的 [code] 转义成 \[code]，禁用后字面保留。
//   - BulletListMarker "*"：对齐 Rust html2md（默认 "-"）与下游 build_api_index 的 "* 描述/参数" 锚点。
//
// 这同时贴合 Rust html2md（渲染表格、列表用 *、极少转义）与下游解析契约。
func htmlToMarkdown(htmlStr string) (string, error) {
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(commonmark.WithBulletListMarker("*")),
			table.NewTablePlugin(),
		),
		converter.WithEscapeMode(converter.EscapeModeDisabled),
	)
	return conv.ConvertString(htmlStr)
}

// -----------------------------------------------------------------------
// Plain 模式：索引页 <a href> 提取
// -----------------------------------------------------------------------

// extractLinks 字节扫描提取 href（对齐 extract_links）。无 DOM；不认 <base href>/data-href；
// 对 mc.163.com 静态索引页够用。过滤见 normalizeHref（跳锚点/非 http scheme/外站，仅留 .html/.htm/.md）。
func extractLinks(htmlStr, baseURL string) []string {
	baseHost, baseDir := splitBase(baseURL)
	var out []string
	const needle = "href"
	i := 0
	for i+4 < len(htmlStr) {
		if !strings.HasPrefix(htmlStr[i:], needle) {
			i++
			continue
		}
		// 'href' 后找 '='（允许空白）。
		j := i + 4
		for j < len(htmlStr) && (htmlStr[j] == ' ' || htmlStr[j] == '\t') {
			j++
		}
		if j >= len(htmlStr) || htmlStr[j] != '=' {
			i++
			continue
		}
		j++
		for j < len(htmlStr) && (htmlStr[j] == ' ' || htmlStr[j] == '\t') {
			j++
		}
		if j >= len(htmlStr) {
			break
		}
		quote := htmlStr[j]
		if quote != '"' && quote != '\'' {
			// 无引号 href；不值得复杂处理。
			i = j
			continue
		}
		j++
		start := j
		for j < len(htmlStr) && htmlStr[j] != quote {
			j++
		}
		if j >= len(htmlStr) {
			break
		}
		seg := htmlStr[start:j]
		if !utf8.ValidString(seg) {
			i = j + 1
			continue
		}
		href := strings.TrimSpace(seg)
		i = j + 1
		if abs, ok := normalizeHref(href, baseHost, baseDir); ok {
			if !containsStr(out, abs) {
				out = append(out, abs)
			}
		}
	}
	return out
}

// splitBase 把 base URL 拆成 (host_with_scheme, dir_with_trailing_slash)（对齐 split_base）。
// 例："https://mc.163.com/dev/mcmanual/mc-dev/" → ("https://mc.163.com", "/dev/mcmanual/mc-dev/")。
func splitBase(baseURL string) (string, string) {
	schemeEnd := strings.Index(baseURL, "://")
	if schemeEnd < 0 {
		return "", "/"
	}
	afterScheme := schemeEnd + 3
	pathRel := strings.IndexByte(baseURL[afterScheme:], '/')
	if pathRel < 0 {
		// 无 path 段 → 根。
		return baseURL, "/"
	}
	hostWithScheme := baseURL[:afterScheme+pathRel]
	path := baseURL[afterScheme+pathRel:]
	// 截到目录（最后一个 '/' 含）。
	idx := strings.LastIndexByte(path, '/')
	var dir string
	switch {
	case idx == 0:
		dir = "/"
	case idx > 0:
		dir = path[:idx+1]
	default:
		dir = "/"
	}
	return hostWithScheme, dir
}

// normalizeHref 当 href 非空/非锚点/非 http 外协议、解析到与 base 同 host、且后缀是
// .html/.htm/.md 时返回 (绝对URL, true)（对齐 normalize_href）。
func normalizeHref(href, baseHostWithScheme, baseDir string) (string, bool) {
	if href == "" {
		return "", false
	}
	if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") ||
		strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") {
		return "", false
	}
	// 过滤/存储前剥 fragment + query。
	clean := href
	if k := strings.IndexByte(clean, '#'); k >= 0 {
		clean = clean[:k]
	}
	if k := strings.IndexByte(clean, '?'); k >= 0 {
		clean = clean[:k]
	}
	if clean == "" {
		return "", false
	}

	var resolved string
	switch {
	case strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://"):
		// 同 host 校验。
		if !strings.HasPrefix(clean, baseHostWithScheme) {
			return "", false
		}
		resolved = clean
	case strings.HasPrefix(clean, "//"):
		// 协议相对 —— 采用 base scheme。
		rest := clean[2:]
		scheme := baseHostWithScheme
		if k := strings.Index(scheme, "://"); k >= 0 {
			scheme = scheme[:k]
		}
		abs := scheme + "://" + rest
		if !strings.HasPrefix(abs, baseHostWithScheme) {
			return "", false
		}
		resolved = abs
	case strings.HasPrefix(clean, "/"):
		// 站点绝对路径。
		resolved = baseHostWithScheme + "/" + clean[1:]
	default:
		// 相对路径；拼到 base 目录下。
		resolved = baseHostWithScheme + baseDir + clean
	}

	lower := strings.ToLower(resolved)
	if !(strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") || strings.HasSuffix(lower, ".md")) {
		return "", false
	}
	return resolved, true
}

// urlToSubpathPlain（Plain 模式）把 URL 映射成相对 .md 子路径，逐字保留 URL 路径段（对齐
// url_to_subpath_plain）。返回正斜杠串，调用方再 filepath.FromSlash 拼到 TargetDir。
func urlToSubpathPlain(rawURL string) string {
	afterScheme := 0
	if k := strings.Index(rawURL, "://"); k >= 0 {
		afterScheme = k + 3
	}
	rest := rawURL[afterScheme:]
	pathStart := strings.IndexByte(rest, '/')
	if pathStart < 0 {
		pathStart = len(rest)
	}
	rawPath := rest[pathStart:]
	trimmed := strings.TrimLeft(rawPath, "/")
	if trimmed == "" {
		return "index.md"
	}

	var segs []string
	lastWasDir := false
	for _, seg := range strings.Split(trimmed, "/") {
		if seg == "" {
			lastWasDir = true
			continue
		}
		segs = append(segs, seg)
		lastWasDir = false
	}
	if lastWasDir {
		segs = append(segs, "index.md")
		return strings.Join(segs, "/")
	}
	segs[len(segs)-1] = rewriteToMarkdownExt(segs[len(segs)-1])
	return strings.Join(segs, "/")
}

// rewriteToMarkdownExt 把末段 .html/.htm 改 .md，.md 原样，其它补 .md（共享给 plain/vuepress）。
func rewriteToMarkdownExt(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".html"):
		return name[:len(name)-len(".html")] + ".md"
	case strings.HasSuffix(lower, ".htm"):
		return name[:len(name)-len(".htm")] + ".md"
	case strings.HasSuffix(lower, ".md"):
		return name
	default:
		return name + ".md"
	}
}

// -----------------------------------------------------------------------
// VuePress 模式：app.js 路由发现 + 内容抽取
// -----------------------------------------------------------------------

// discoverVuepressRoutes 做 VuePress 1.x 路由发现，返回 [absURL, routePath] 对（对齐
// discover_vuepress_routes）。只留 .html 叶子（VuePress 也为 SPA 根发 "/xxx/" 目录路由）。
func discoverVuepressRoutes(ctx context.Context, client *http.Client, baseURL string) ([][2]string, error) {
	// 1. 抓 SPA 壳 HTML，学到带 hash 的 app.js URL。
	indexHTML, err := fetchText(ctx, client, baseURL)
	if err != nil {
		return nil, fmt.Errorf("fetch vuepress index: %w", err)
	}
	appJSPath, ok := extractAppJSURL(indexHTML)
	if !ok {
		return nil, fmt.Errorf("app.js URL not found in %s", baseURL)
	}
	appJSURL := absoluteAppJSURL(baseURL, appJSPath)

	// 2. 拉 bundle（~MB）。
	appJS, err := fetchText(ctx, client, appJSURL)
	if err != nil {
		return nil, fmt.Errorf("fetch app.js: %w", err)
	}

	// 3. 还原路由；只留 .html 叶子。
	routes := extractVuepressRoutes(appJS)
	pairs := make([][2]string, 0, len(routes))
	for _, route := range routes {
		if !strings.HasSuffix(route, ".html") {
			continue
		}
		abs := absoluteURLFromRoute(baseURL, route)
		pairs = append(pairs, [2]string{abs, route})
	}
	return pairs, nil
}

// extractAppJSURL 找 SPA 壳里引用的带 hash app.<hash>.js URL（对齐 extract_app_js_url）。
// 要求落在 /.../assets/js/ 下且 app. 与 .js 间是 hex hash。<link href> 与内联裸引用都覆盖。
func extractAppJSURL(htmlStr string) (string, bool) {
	const needle = "app."
	const suffix = ".js"
	pos := 0
	for {
		rel := strings.Index(htmlStr[pos:], needle)
		if rel < 0 {
			break
		}
		start := pos + rel
		afterApp := start + len(needle)
		relJS := strings.Index(htmlStr[afterApp:], suffix)
		if relJS < 0 {
			break
		}
		endJS := afterApp + relJS + len(suffix)
		if endJS > len(htmlStr) {
			break
		}
		hash := htmlStr[afterApp : afterApp+relJS]
		// hash 非空、≤64、全 hex（注释说 4..32 typically，但代码只校验这三条 —— 照代码，R5）。
		hexOK := hash != "" && len(hash) <= 64 && allASCIIHex(hash)
		if !hexOK {
			pos = start + 1
			continue
		}
		// 从 start 向左回退到 " / ' / ( / 空格 / 换行 边界。
		left := start
		for left > 0 {
			c := htmlStr[left-1]
			if c == '"' || c == '\'' || c == '(' || c == ' ' || c == '\n' || c == '\r' {
				break
			}
			left--
		}
		candidate := htmlStr[left:endJS]
		// 必须像站点绝对路径。
		if pathStart := strings.IndexByte(candidate, '/'); pathStart >= 0 {
			path := candidate[pathStart:]
			if strings.Contains(path, "/assets/js/") && strings.HasSuffix(path, ".js") {
				return path, true
			}
		}
		pos = endJS
	}
	return "", false
}

func allASCIIHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// absoluteAppJSURL 把 app.js 路径解析到 base URL 的 host 下（对齐 absolute_app_js_url）。
func absoluteAppJSURL(baseURL, appJSPath string) string {
	if strings.HasPrefix(appJSPath, "http://") || strings.HasPrefix(appJSPath, "https://") {
		return appJSPath
	}
	hostEnd := 0
	if k := strings.Index(baseURL, "://"); k >= 0 {
		hostEnd = k + 3
	}
	pathStart := len(baseURL)
	if k := strings.IndexByte(baseURL[hostEnd:], '/'); k >= 0 {
		pathStart = hostEnd + k
	}
	hostWithScheme := baseURL[:pathStart]
	if strings.HasPrefix(appJSPath, "/") {
		return hostWithScheme + "/" + appJSPath[1:]
	}
	return hostWithScheme + "/" + appJSPath
}

// extractVuepressRoutes 扫 bundle 里的 path:"/xxx/" 字面（对齐 extract_vuepress_routes）。
// VuePress 1.x 把路由配置编进 bundle，每条 path 即页面解析的规范全路径。去重 + 排序。
func extractVuepressRoutes(appJS string) []string {
	const needle = `path:"`
	var result []string
	pos := 0
	for {
		start := strings.Index(appJS[pos:], needle)
		if start < 0 {
			break
		}
		after := pos + start + len(needle)
		endRel := strings.IndexByte(appJS[after:], '"')
		if endRel < 0 {
			break
		}
		raw := appJS[after : after+endRel]
		if strings.HasPrefix(raw, "/") {
			if decoded, ok := percentDecode(raw); ok {
				if !containsStr(result, decoded) {
					result = append(result, decoded)
				}
			}
		}
		pos = after + endRel + 1
	}
	sort.Strings(result)
	return result
}

// percentDecode 是带 UTF-8 / GBK 回退的 RFC 3986 百分号解码（对齐 percent_decode）。
// mc.163.com 路由表一半 UTF-8 一半 GBK 百分号编码，直 UTF-8 解会丢掉 GBK 那半。先 UTF-8，
// 失败回退 GBK；仅当两者都失败（如末尾截断的 hex）才返回 false。无 '+' → 空格改写（路径非表单）。
func percentDecode(s string) (string, bool) {
	b := []byte(s)
	out := make([]byte, 0, len(b))
	i := 0
	for i < len(b) {
		if b[i] == '%' && i+2 < len(b) {
			hi := fromHexDigit(b[i+1])
			lo := fromHexDigit(b[i+2])
			if hi < 0 || lo < 0 {
				return "", false
			}
			out = append(out, byte(hi<<4|lo))
			i += 3
		} else {
			out = append(out, b[i])
			i++
		}
	}
	if utf8.Valid(out) {
		return string(out), true
	}
	// 回退 GBK：仅在无替换字符（U+FFFD）时接受，否则会静默损坏路径（对齐 Rust !had_errors）。
	dec, err := simplifiedchinese.GBK.NewDecoder().Bytes(out)
	if err != nil {
		return "", false
	}
	if bytes.ContainsRune(dec, utf8.RuneError) {
		return "", false
	}
	return string(dec), true
}

func fromHexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// absoluteURLFromRoute 用 base 的 scheme+host+祖先目录 + route 叶子路径拼绝对页 URL（对齐
// absolute_url_from_route）。定位 route 首段（如 mcdocs）在 base 中的位置；定位不到回退 host-only。
func absoluteURLFromRoute(baseURL, route string) string {
	// 取首段（"/mcdocs" / "/mcguide" / ...）。
	parts := strings.Split(route, "/")
	if len(parts) < 2 || parts[1] == "" {
		// route 不是 "/leaf/..." 形状；直接 host + route。
		return joinHostAndPath(baseURL, route)
	}
	marker := "/" + parts[1]
	if idx := strings.Index(baseURL, marker); idx >= 0 {
		return baseURL[:idx] + route
	}
	return joinHostAndPath(baseURL, route)
}

func joinHostAndPath(baseURL, path string) string {
	hostEnd := 0
	if k := strings.Index(baseURL, "://"); k >= 0 {
		hostEnd = k + 3
	}
	hostSlash := len(baseURL)
	if k := strings.IndexByte(baseURL[hostEnd:], '/'); k >= 0 {
		hostSlash = hostEnd + k
	}
	hostPart := baseURL[:hostSlash]
	if strings.HasPrefix(path, "/") {
		return hostPart + "/" + path[1:]
	}
	return hostPart + "/" + path
}

// vuepressRouteToSubpath 把 VuePress route（如 "/mcdocs/foo/bar.html"）转成 TargetDir 下相对
// .md 路径（对齐 vuepress_route_to_subpath）。若 TargetDir 末段与 route 首段同（都 mcdocs）则剥首段，
// 否则保全路径。末段 .html/.htm → .md。返回正斜杠串。
func vuepressRouteToSubpath(route, targetDir string) string {
	trimmed := strings.TrimLeft(route, "/")
	if trimmed == "" {
		return "index.md"
	}
	targetLeaf := filepath.Base(targetDir)
	var segments []string
	for _, s := range strings.Split(trimmed, "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}
	if targetLeaf != "" && len(segments) > 0 && segments[0] == targetLeaf {
		segments = segments[1:]
	}
	if len(segments) == 0 {
		return "index.md"
	}
	lastIdx := len(segments) - 1
	out := make([]string, len(segments))
	for i, seg := range segments {
		if i == lastIdx {
			out[i] = rewriteToMarkdownExt(seg)
		} else {
			out[i] = seg
		}
	}
	return strings.Join(out, "/")
}

// extractVuepressContent 从 VuePress 渲染页切出文章体：<h1> 到首个页尾标记（page-info-hide /
// edit-link / page-nav / page-edit / <footer）之前，再剥 header anchor、把 custom-block 改 blockquote
// （对齐 extract_vuepress_content）。
func extractVuepressContent(htmlStr string) (string, bool) {
	h1Start := strings.Index(htmlStr, "<h1")
	if h1Start < 0 {
		return "", false
	}
	endPos := len(htmlStr)
	markers := []string{
		`class="page-info-hide"`,
		`class="edit-link"`,
		`class="page-nav"`,
		`class="page-edit"`,
		"<footer",
	}
	for _, marker := range markers {
		rel := strings.Index(htmlStr[h1Start:], marker)
		if rel < 0 {
			continue
		}
		abs := h1Start + rel
		// 回退到最近的 '<'，避免切在标签中间。
		if bracket := strings.LastIndexByte(htmlStr[h1Start:abs], '<'); bracket >= 0 {
			safe := h1Start + bracket
			if safe > h1Start && safe < endPos {
				endPos = safe
			}
		} else if abs < endPos {
			endPos = abs
		}
	}
	if endPos <= h1Start {
		return "", false
	}
	content := htmlStr[h1Start:endPos]
	cleaned := stripHeaderAnchors(content)
	cleaned = transformCustomBlocks(cleaned)
	return cleaned, true
}

// stripHeaderAnchors 删 <a ... class="header-anchor" ...>...</a> 块（VuePress 在标题旁发的 # 永久链）
// （对齐 strip_header_anchors）。字节扫 <a；含 header-anchor 的整段丢弃，否则原样保留（无嵌套处理）。
func stripHeaderAnchors(htmlStr string) string {
	var out strings.Builder
	out.Grow(len(htmlStr))
	i := 0
	for i < len(htmlStr) {
		rel := strings.Index(htmlStr[i:], "<a")
		if rel < 0 {
			out.WriteString(htmlStr[i:])
			break
		}
		openStart := i + rel
		out.WriteString(htmlStr[i:openStart])
		openEndRel := strings.IndexByte(htmlStr[openStart:], '>')
		if openEndRel < 0 {
			out.WriteString(htmlStr[openStart:])
			break
		}
		openEnd := openStart + openEndRel + 1
		openTag := htmlStr[openStart:openEnd]
		closeRel := strings.Index(htmlStr[openEnd:], "</a>")
		var dropUntil int
		var hasAnchorClass bool
		if closeRel >= 0 {
			dropUntil = openEnd + closeRel + len("</a>")
			hasAnchorClass = strings.Contains(openTag, "header-anchor")
		} else {
			// 无闭合标签；保留剩余原样后收尾。
			dropUntil = len(htmlStr)
			hasAnchorClass = false
		}
		if hasAnchorClass {
			i = dropUntil
		} else {
			out.WriteString(htmlStr[openStart:dropUntil])
			i = dropUntil
		}
	}
	return out.String()
}

// transformCustomBlocks 把 VuePress custom-block 容器改成纯 <blockquote>（对齐 transform_custom_blocks）。
// 仅处理 tip/warning/danger，其它原样穿透。
func transformCustomBlocks(htmlStr string) string {
	const openerPrefix = `<div class="custom-block `
	var out strings.Builder
	out.Grow(len(htmlStr))
	i := 0
	for i < len(htmlStr) {
		rel := strings.Index(htmlStr[i:], openerPrefix)
		if rel < 0 {
			out.WriteString(htmlStr[i:])
			break
		}
		openStart := i + rel
		out.WriteString(htmlStr[i:openStart])

		// 识别 flavor。
		afterPrefix := openStart + len(openerPrefix)
		relCloseQuote := strings.IndexByte(htmlStr[afterPrefix:], '"')
		if relCloseQuote < 0 {
			out.WriteString(htmlStr[openStart:])
			break
		}
		flavor := htmlStr[afterPrefix : afterPrefix+relCloseQuote]
		// 定位 opening <div ...> 标签结束。
		relCloseDiv := strings.IndexByte(htmlStr[afterPrefix:], '>')
		if relCloseDiv < 0 {
			out.WriteString(htmlStr[openStart:])
			break
		}
		openerEnd := afterPrefix + relCloseDiv + 1
		if flavor != "tip" && flavor != "warning" && flavor != "danger" {
			// 穿透这个 opener，从其后继续扫。
			out.WriteString(htmlStr[openStart:openerEnd])
			i = openerEnd
			continue
		}

		// 找匹配 </div>（允许嵌套）。
		bodyStart := openerEnd
		closePos, ok := findMatchingDivClose(htmlStr, bodyStart)
		if !ok {
			out.WriteString(htmlStr[openStart:openerEnd])
			i = openerEnd
			continue
		}
		body := htmlStr[bodyStart:closePos]
		closeEnd := closePos + len("</div>")

		// 可选标题抽取。
		const titleMarker = `<p class="custom-block-title">`
		var title string
		hasTitle := false
		bodyAfterTitle := body
		if tpos := strings.Index(body, titleMarker); tpos >= 0 {
			titleOpenEnd := tpos + len(titleMarker)
			if relPClose := strings.Index(body[titleOpenEnd:], "</p>"); relPClose >= 0 {
				title = body[titleOpenEnd : titleOpenEnd+relPClose]
				hasTitle = true
				after := titleOpenEnd + relPClose + len("</p>")
				bodyAfterTitle = body[after:]
			}
		}

		out.WriteString("<blockquote>")
		if hasTitle {
			out.WriteString("<strong>")
			out.WriteString(strings.TrimSpace(title))
			out.WriteString("</strong>")
		}
		out.WriteString(bodyAfterTitle)
		out.WriteString("</blockquote>")
		i = closeEnd
	}
	return out.String()
}

// findMatchingDivClose 给一个指向 <div ...> 开标签之后的 start，返回匹配 </div> 的 '<' 字节索引
// （对齐 find_matching_div_close）。处理任意嵌套 <div>。
func findMatchingDivClose(htmlStr string, start int) (int, bool) {
	depth := 1
	i := start
	for i < len(htmlStr) {
		nextOpen := -1
		if r := strings.Index(htmlStr[i:], "<div"); r >= 0 {
			nextOpen = i + r
		}
		nextClose := -1
		if r := strings.Index(htmlStr[i:], "</div>"); r >= 0 {
			nextClose = i + r
		}
		switch {
		case nextOpen >= 0 && nextClose >= 0 && nextOpen < nextClose:
			depth++
			i = nextOpen + len("<div")
		case nextClose >= 0:
			depth--
			if depth == 0 {
				return nextClose, true
			}
			i = nextClose + len("</div>")
		default:
			return 0, false
		}
	}
	return 0, false
}

// rewriteInternalLinks 把绝对上游链接改写成离线可用的相对链接（对齐 rewrite_internal_links）。
// 仅触发 (/dev/mcmanual/mc-dev/{mcdocs,mcguide,mconline}/：同源 → (./path.md)，异源 → (../src/path.md)，
// 保留尾随 #fragment。
func rewriteInternalLinks(md, currentSource string) string {
	const prefix = "(/dev/mcmanual/mc-dev/"
	allowed := map[string]bool{"mcdocs": true, "mcguide": true, "mconline": true}
	var out strings.Builder
	out.Grow(len(md))
	i := 0
	for i < len(md) {
		rel := strings.Index(md[i:], prefix)
		if rel < 0 {
			out.WriteString(md[i:])
			break
		}
		start := i + rel
		out.WriteString(md[i:start])
		afterPrefix := start + len(prefix)
		// 找 source 段（到下一个 '/'）。
		relSlash := strings.IndexByte(md[afterPrefix:], '/')
		if relSlash < 0 {
			out.WriteString(md[start:afterPrefix])
			i = afterPrefix
			continue
		}
		src := md[afterPrefix : afterPrefix+relSlash]
		if !allowed[src] {
			out.WriteString(md[start:afterPrefix])
			i = afterPrefix
			continue
		}
		restStart := afterPrefix + relSlash + 1 // 在 <src>/ 之后
		relClose := strings.IndexByte(md[restStart:], ')')
		if relClose < 0 {
			out.WriteString(md[start:restStart])
			i = restStart
			continue
		}
		closeIdx := restStart + relClose
		inner := md[restStart:closeIdx] // 如 "foo/bar.html#sec"
		// 拆出可选 fragment。
		var pathPart, fragment string
		if p := strings.IndexByte(inner, '#'); p >= 0 {
			pathPart = inner[:p]
			fragment = inner[p:]
		} else {
			pathPart = inner
			fragment = ""
		}
		// 末尾 .html/.htm → .md（其它原样，含已 .md）。
		lower := strings.ToLower(pathPart)
		var pathMd string
		switch {
		case strings.HasSuffix(lower, ".html"):
			pathMd = pathPart[:len(pathPart)-len(".html")] + ".md"
		case strings.HasSuffix(lower, ".htm"):
			pathMd = pathPart[:len(pathPart)-len(".htm")] + ".md"
		default:
			pathMd = pathPart
		}
		var rewritten string
		if src == currentSource {
			rewritten = "(./" + pathMd + fragment + ")"
		} else {
			rewritten = "(../" + src + "/" + pathMd + fragment + ")"
		}
		out.WriteString(rewritten)
		i = closeIdx + 1
	}
	return out.String()
}

// containsStr 线性查找（保插入顺序去重用，对齐 Rust Vec::contains）。
func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
