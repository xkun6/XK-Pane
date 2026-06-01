package webapi

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strings"
)

// webMarker 是网关注入到所托管 HTML 的「Web 端」标记。浏览器经本网关加载页面时置
// window.__XKPANE_WEB__=true，前端 transport 探针据此判定 viewer；桌面 webview 经 Wails
// AssetServer 加载（不经本网关）无此注入 → owner。标记在「服务时」注入、不写入磁盘文件，
// 故桌面 webview 永远看不到它，跨平台无歧义（不依赖 Wails 运行时内部 _wails.* 的注入时序）。
// 用 classic inline script（非 module）放在 </head> 前 → 先于 deferred 的 bundle 模块执行。
const webMarker = `<script>window.__XKPANE_WEB__=true;</script>`

// buildMux 装配路由：/api/health 与 /pair 公开，其余一律走 requirePaired 闸门（持有效 cookie）。
// Go 1.22+ ServeMux 的精确路径/方法比 "/" catch-all 更优先，故无需手写顺序判断。
// 远程能力是 fail-closed 白名单（§9）：只挂 launch + 只读列表 + /ws，红线方法绝不注册。
func (g *Gateway) buildMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /pair", g.handlePair)

	// 会话编排 REST（M3 只读镜像：launch + 只读列表；input/resize/kill = M5/M6）。
	mux.Handle("POST /api/launch", g.requirePaired(http.HandlerFunc(g.handleLaunch)))
	mux.Handle("POST /api/sessions/input", g.requirePaired(http.HandlerFunc(g.handleSessionInput)))
	mux.Handle("GET /api/sessions", g.requirePaired(http.HandlerFunc(g.handleSessions)))
	mux.Handle("GET /api/sessions/scan", g.requirePaired(http.HandlerFunc(g.handleSessionsScan)))
	mux.Handle("GET /api/sessions/replay", g.requirePaired(http.HandlerFunc(g.handleSessionReplay)))
	mux.Handle("GET /api/sessions/timeline", g.requirePaired(http.HandlerFunc(g.handleSessionTimeline)))
	mux.Handle("GET /api/projects", g.requirePaired(http.HandlerFunc(g.handleProjects)))
	mux.Handle("GET /api/workspaces", g.requirePaired(http.HandlerFunc(g.handleWorkspaces)))
	// 实时事件镜像（终端输出/状态/退出 + 编排 launch）。
	mux.Handle("GET /ws", g.requirePaired(http.HandlerFunc(g.handleWS)))

	// @wailsio/runtime 在浏览器模式会探测 /wails/custom.js 与 POST /wails/runtime（桌面端注入项）。
	// 网关上没有这些 → 显式 404，避免落到 SPA 回退返回 HTML（被当 JS 解析报 Unexpected token '<'，
	// 且让未移植的 binding 调用快速失败而非拿到 HTML 200）。Web 端这些调用本就该走 transport HTTP。
	mux.HandleFunc("/wails/", http.NotFound)

	mux.Handle("/", g.requirePaired(g.staticHandler()))
	return mux
}

// handleHealth 无鉴权探活。
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handlePair 校验 ?token= → 下发 httpOnly Session Cookie → 重定向 /（已带 cookie，过闸门）。
func (g *Gateway) handlePair(w http.ResponseWriter, r *http.Request) {
	got := r.URL.Query().Get("token")
	want := g.currentToken()
	if want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, unpairedHTML)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    g.sessionCookieValue(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600, // 30 天
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// requirePaired 是闸门：持有效 cookie 才放行，否则 401 + 配对指引页。
func (g *Gateway) requirePaired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.validCookie(r) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, unpairedHTML)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// staticHandler 托管内嵌 frontend/dist：命中文件直接发，未命中回退 index.html（SPA）。
func (g *Gateway) staticHandler() http.Handler {
	fileServer := http.FileServer(http.FS(g.assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		// 任何会落到 index.html 的请求（根 / SPA 回退 / 直接 /index.html）都走 serveIndex，
		// 确保浏览器拿到的 HTML 一定带 webMarker（直接 fileServer 发原始文件会漏标记）。
		if clean == "" || clean == "." || clean == "index.html" {
			g.serveIndex(w)
			return
		}
		f, err := g.assets.Open(clean)
		if err != nil {
			g.serveIndex(w) // 未命中文件 → SPA 回退
			return
		}
		_ = f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

// serveIndex 读出内嵌 index.html、注入 webMarker（</head> 前），以 text/html 写回
// （SPA 入口 / 回退）。注入让经本网关访问的浏览器被前端探针判定为 viewer。
func (g *Gateway) serveIndex(w http.ResponseWriter) {
	f, err := g.assets.Open("index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "read index.html failed", http.StatusInternalServerError)
		return
	}
	if i := bytes.Index(data, []byte("</head>")); i >= 0 {
		out := make([]byte, 0, len(data)+len(webMarker))
		out = append(out, data[:i]...)
		out = append(out, webMarker...)
		out = append(out, data[i:]...)
		data = out
	} else {
		// 兜底：无 </head>（不应发生）→ 前置，确保仍先于 bundle 模块执行。
		data = append([]byte(webMarker), data...)
	}
	// index.html 绝不缓存：哈希命名的 chunk 可长缓存，但若 index.html 被缓存为旧版、其引用的旧
	// chunk 哈希在重建后的二进制里已不存在 → chunk 404 → SPA 加载失败（纯色空白）。no-store 让每次
	// 都取最新 index，杜绝「陈旧 index 指向死 chunk」。
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// unpairedHTML 是未配对时的极简指引页（后端兜底页，非 SPA，i18n 铁律只约束前端 React）。
const unpairedHTML = `<!doctype html><html><head><meta charset="utf-8">` +
	`<meta name="viewport" content="width=device-width,initial-scale=1">` +
	`<title>XK-Pane</title></head>` +
	`<body style="font-family:system-ui;display:flex;align-items:center;justify-content:center;` +
	`height:100vh;margin:0;background:#1e1e1e;color:#ddd;text-align:center">` +
	`<div><h2>需要配对 / Pairing required</h2>` +
	`<p>请在桌面端「设置 → Web 访问」扫码配对后访问。</p>` +
	`<p>Scan the QR code in the desktop app (Settings → Web Access) to pair.</p></div>` +
	`</body></html>`
