package orchestrator

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 注：HTTP+MCP Streamable HTTP 全链路（含真实 Claude 连通 + tools/call）已由 P6a spike
// （cmd/mcp-spike，已删）+ 用户真实 CLI 手测验证。此处单测覆盖安全关键的鉴权中间件与纯函数。

func TestAuthMiddleware(t *testing.T) {
	const tok = "deadbeefcafe0000"
	newH := func() (http.Handler, *bool, *string) {
		called := false
		accept := ""
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			accept = r.Header.Get("Accept")
			w.WriteHeader(http.StatusOK)
		})
		return authMiddleware(tok, next), &called, &accept
	}

	t.Run("正确 Bearer header 放行 + 注入 Accept", func(t *testing.T) {
		h, called, accept := newH()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !*called {
			t.Fatalf("应放行：code=%d called=%v", rec.Code, *called)
		}
		if !strings.Contains(*accept, "text/event-stream") {
			t.Errorf("应注入 Accept text/event-stream，得 %q", *accept)
		}
	})

	t.Run("?token= query 后备放行（Claude Issue #7290）", func(t *testing.T) {
		h, called, _ := newH()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp?token="+tok, nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !*called {
			t.Fatalf("query token 应放行：code=%d", rec.Code)
		}
	})

	t.Run("无 token → 401 且不调用 next", func(t *testing.T) {
		h, called, _ := newH()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("无 token 应 401，得 %d", rec.Code)
		}
		if *called {
			t.Errorf("无 token 不应调用 next")
		}
	})

	t.Run("错误 token → 401", func(t *testing.T) {
		h, _, _ := newH()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("错误 token 应 401，得 %d", rec.Code)
		}
	})

	t.Run("OPTIONS 跳过鉴权（CORS 预检）", func(t *testing.T) {
		h, called, _ := newH()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
		h.ServeHTTP(rec, req)
		if !*called {
			t.Errorf("OPTIONS 应放行到 next")
		}
	})

	t.Run("已带 text/event-stream 的 Accept 不被覆盖丢失", func(t *testing.T) {
		h, _, accept := newH()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "text/event-stream")
		h.ServeHTTP(rec, req)
		if !strings.Contains(*accept, "text/event-stream") {
			t.Errorf("既有 Accept 应保留，得 %q", *accept)
		}
	})
}

func TestBearer(t *testing.T) {
	cases := map[string]string{
		"Bearer abc":  "abc",
		"Bearer  abc": "abc", // TrimSpace
		"bearer abc":  "",    // 大小写敏感
		"abc":         "",
		"":            "",
	}
	for in, want := range cases {
		if got := bearer(in); got != want {
			t.Errorf("bearer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenToken(t *testing.T) {
	a, b := genToken(), genToken()
	if len(a) != 32 {
		t.Errorf("token 长度应为 32 hex，得 %d", len(a))
	}
	if a == b {
		t.Errorf("两次 genToken 不应相同")
	}
	for _, c := range a {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("token 含非 hex 字符: %q", a)
		}
	}
}

func TestBuildMCPHandlerNonNil(t *testing.T) {
	// 注册阶段不触 deps（仅 handler 调用时才用），故 New(Deps{}) 空依赖即可校验 handler 构造。
	if New(Deps{}).buildMCPHandler() == nil {
		t.Fatal("buildMCPHandler 返回 nil")
	}
}
