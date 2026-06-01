package webapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// newTestGateway 构造一个不接 DB、token 固定的网关（直接设字段，测 HTTP 行为）。
func newTestGateway(token string) *Gateway {
	g := &Gateway{
		assets: fstest.MapFS{
			"index.html":    {Data: []byte("<!doctype html><title>spa</title>")},
			"assets/app.js": {Data: []byte("console.log('app')")},
		},
		hub:   newHub(),
		port:  DefaultPort,
		token: token,
	}
	return g
}

func TestHealthPublic(t *testing.T) {
	g := newTestGateway("tok")
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}
}

func TestStaticGatedUntilPaired(t *testing.T) {
	g := newTestGateway("secret-token")
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	// 不带 cookie 的客户端（禁止自动跟随重定向，便于断言 302）。
	noJar := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	// 1) 未配对访问 / → 401。
	resp, err := noJar.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unpaired / status = %d, want 401", resp.StatusCode)
	}

	// 2) 错误 token 配对 → 401。
	resp, err = noJar.Get(srv.URL + "/pair?token=wrong")
	if err != nil {
		t.Fatalf("pair wrong: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("pair wrong status = %d, want 401", resp.StatusCode)
	}

	// 3) 正确 token 配对 → 302 + Set-Cookie。
	resp, err = noJar.Get(srv.URL + "/pair?token=secret-token")
	if err != nil {
		t.Fatalf("pair ok: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("pair ok status = %d, want 302", resp.StatusCode)
	}
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("pair ok did not set session cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}

	// 4) 带 cookie 访问 / → 200 + SPA。
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	req.AddCookie(sessionCookie)
	resp, err = noJar.Do(req)
	if err != nil {
		t.Fatalf("get / with cookie: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("paired / status = %d, want 200", resp.StatusCode)
	}
}

func TestForgedCookieRejected(t *testing.T) {
	g := newTestGateway("real-token")
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "deadbeef"})
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("forged cookie status = %d, want 401", resp.StatusCode)
	}
}

func TestSessionCookieValueBindsToToken(t *testing.T) {
	a := newTestGateway("token-a")
	b := newTestGateway("token-b")
	if a.sessionCookieValue() == b.sessionCookieValue() {
		t.Fatal("cookie value must differ across tokens")
	}
	if a.sessionCookieValue() != newTestGateway("token-a").sessionCookieValue() {
		t.Fatal("cookie value must be deterministic for a fixed token")
	}
}

func TestStaticServesAssetWithCookie(t *testing.T) {
	g := newTestGateway("tok")
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/assets/app.js", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: g.sessionCookieValue()})
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("asset status = %d, want 200", resp.StatusCode)
	}
}

// TestServeIndexInjectsWebMarker：经网关托管的 HTML 带 window.__XKPANE_WEB__ 标记（前端探针
// 据此判定 viewer），且原始静态资源不带（标记仅注入到 index.html）。防回归：曾用 '_wails in window'
// 误判（@wailsio/runtime 在浏览器也设 _wails）导致整条 Web 路径失效。
func TestServeIndexInjectsWebMarker(t *testing.T) {
	g := &Gateway{
		assets: fstest.MapFS{
			"index.html":    {Data: []byte("<!doctype html><html><head><title>x</title></head><body>app</body></html>")},
			"assets/app.js": {Data: []byte("console.log('app')")},
		},
		hub:   newHub(),
		port:  DefaultPort,
		token: "tok",
	}
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	get := func(path string) string {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		req.AddCookie(&http.Cookie{Name: cookieName, Value: g.sessionCookieValue()})
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		defer resp.Body.Close()
		b := make([]byte, 4096)
		n, _ := resp.Body.Read(b)
		return string(b[:n])
	}

	// 1) / 与 /index.html 都含标记，且注入在 </head> 之前。
	for _, p := range []string{"/", "/index.html"} {
		html := get(p)
		if !strings.Contains(html, "window.__XKPANE_WEB__") {
			t.Fatalf("%s missing web marker: %q", p, html)
		}
		mi := strings.Index(html, "window.__XKPANE_WEB__")
		hi := strings.Index(html, "</head>")
		if hi < 0 || mi > hi {
			t.Fatalf("%s marker must precede </head> (marker@%d head@%d)", p, mi, hi)
		}
	}

	// 2) 原始 JS 资源不含标记。
	if js := get("/assets/app.js"); strings.Contains(js, "__XKPANE_WEB__") {
		t.Fatalf("asset must not carry web marker: %q", js)
	}
}

func TestGenTokenUnique(t *testing.T) {
	a, b := genToken(), genToken()
	if a == b {
		t.Fatal("genToken returned identical tokens")
	}
	if len(a) != 48 {
		t.Fatalf("genToken len = %d, want 48", len(a))
	}
}

func TestLanIPv4sNoPanic(t *testing.T) {
	for _, ip := range lanIPv4s() {
		if strings.Count(ip, ".") != 3 {
			t.Fatalf("not an IPv4: %q", ip)
		}
	}
}
