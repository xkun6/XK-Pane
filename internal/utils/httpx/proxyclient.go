package httpx

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// proxyclient.go：构造**独立** *http.Client（不复用单例），支持 per-call 代理 / socks5 / 超时 / UA。
// 给 OTA（bedrockdocsync）这类需走代理的场景用。单例 Client() 保持无代理、不受影响。
//
// 对齐 cc-pane bedrock_doc_sync/proxy.rs build_reqwest：http/https/socks5(h) 统一走
// Transport.Proxy（stdlib 原生支持 socks5/socks5h scheme，域名解析同样在代理侧完成）；
// auth "user:pass" 注入 URL userinfo；超时 + UA 可定制。

// ClientOptions 配置一个独立 http.Client。
type ClientOptions struct {
	// Proxy 代理 URL：http(s)://host:port 或 socks5://host:port。"" = 直连（不用代理）。
	Proxy string
	// ProxyAuth "user:pass" 基础认证；"" = 无（畸形值静默忽略，宁可不认证也别整体失败）。
	ProxyAuth string
	// Timeout 设为 http.Client.Timeout（总超时）。0 = 不设（由 ctx 控制）。
	Timeout time.Duration
	// ConnectTimeout 连接超时。0 = 默认 10s（对齐 reqwest connect_timeout）。
	ConnectTimeout time.Duration
}

// NewClient 按 opts 造一个独立 *http.Client。Proxy URL 非法（无 scheme/host 或不支持的 scheme）返错。
func NewClient(opts ClientOptions) (*http.Client, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	var tr *http.Transport
	if ok {
		tr = base.Clone()
	} else {
		tr = &http.Transport{}
	}
	tr.MaxIdleConns = 100
	tr.MaxIdleConnsPerHost = 16
	tr.IdleConnTimeout = 90 * time.Second

	connTimeout := opts.ConnectTimeout
	if connTimeout <= 0 {
		connTimeout = 10 * time.Second
	}
	dialer := &net.Dialer{Timeout: connTimeout, KeepAlive: 30 * time.Second}
	tr.DialContext = dialer.DialContext

	if p := strings.TrimSpace(opts.Proxy); p != "" {
		u, err := url.Parse(p)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("httpx: invalid proxy url: %q", opts.Proxy)
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "socks5", "socks5h":
			if user, pass, ok := splitProxyAuth(opts.ProxyAuth); ok {
				u.User = url.UserPassword(user, pass)
			}
			tr.Proxy = http.ProxyURL(u)
		default:
			return nil, fmt.Errorf("httpx: unsupported proxy scheme: %q", u.Scheme)
		}
	}

	return &http.Client{Transport: tr, Timeout: opts.Timeout}, nil
}

// splitProxyAuth 解析 "user:pass"。无冒号或为空 → (,,false)（畸形静默忽略）。
func splitProxyAuth(auth string) (string, string, bool) {
	if auth == "" {
		return "", "", false
	}
	i := strings.IndexByte(auth, ':')
	if i < 0 {
		return "", "", false
	}
	return auth[:i], auth[i+1:], true
}
