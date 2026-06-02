package httpx

import (
	"net/http"
	"testing"
)

func transportOf(t *testing.T, c *http.Client) *http.Transport {
	t.Helper()
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport 不是 *http.Transport: %T", c.Transport)
	}
	return tr
}

func TestNewClient_NoProxy(t *testing.T) {
	c, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c == nil || transportOf(t, c).DialContext == nil {
		t.Fatal("应有自定义 DialContext")
	}
}

func TestNewClient_HTTPProxy(t *testing.T) {
	c, err := NewClient(ClientOptions{Proxy: "http://127.0.0.1:7890", ProxyAuth: "user:pass"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	tr := transportOf(t, c)
	if tr.Proxy == nil {
		t.Fatal("http 代理应设 Transport.Proxy")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	u, err := tr.Proxy(req)
	if err != nil || u == nil || u.Host != "127.0.0.1:7890" {
		t.Fatalf("Proxy(req) = %v, %v", u, err)
	}
	if user := u.User.Username(); user != "user" {
		t.Fatalf("代理 auth 用户名应注入，得 %q", user)
	}
}

func TestNewClient_SOCKS5(t *testing.T) {
	c, err := NewClient(ClientOptions{Proxy: "socks5://127.0.0.1:1080"})
	if err != nil {
		t.Fatalf("NewClient socks5: %v", err)
	}
	tr := transportOf(t, c)
	if tr.Proxy != nil {
		t.Fatal("socks5 应走 DialContext 而非 Transport.Proxy")
	}
	if tr.DialContext == nil {
		t.Fatal("socks5 应设 DialContext")
	}
}

func TestNewClient_InvalidProxy(t *testing.T) {
	if _, err := NewClient(ClientOptions{Proxy: "not a url at all"}); err == nil {
		t.Fatal("非法代理 url 应报错")
	}
	if _, err := NewClient(ClientOptions{Proxy: "http://"}); err == nil {
		t.Fatal("无 host 的代理 url 应报错")
	}
	if _, err := NewClient(ClientOptions{Proxy: "ftp://x:21"}); err == nil {
		t.Fatal("不支持的 scheme 应报错")
	}
}
