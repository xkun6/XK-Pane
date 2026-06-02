package bedrockdocsync

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"xkpane/internal/utils/httpx"
)

// proxy.go：HTTPS 代理解析 + 独立 http.Client 构造 + 探测。对齐 proxy.rs。
// 无 git2——OTA 链路刻意无 git（Gitea API + 纯 HTTP），绕开多 GB 浅克隆。

// ProxyConfig 用户代理配置（持久化 bedrock-data/proxy.json）。两字段可空：
// 默认（空 ProxyConfig）= 用系统代理 / env / 直连。
type ProxyConfig struct {
	// HTTPSProxy 代理 URL，如 "http://127.0.0.1:7890" 或 "socks5://..."。空 → 回退 env / 系统。
	HTTPSProxy string `json:"httpsProxy,omitempty"`
	// Auth 可选 "user:pass" 基础认证。
	Auth string `json:"auth,omitempty"`
}

// ProbeResult 是 5s HEAD https://github.com/ 探测结果（UI「测试代理」按钮）。探 github.com 而非
// 实际上游，因它是「出站 HTTPS 经我的代理是否通」最通用的金丝雀。
type ProbeResult struct {
	OK         bool  `json:"ok"`
	StatusCode int   `json:"statusCode"`
	LatencyMs  int64 `json:"latencyMs"`
}

const probeUserAgent = "cc-panes-bedrock-doc-sync/1.0"

// ProxyConfigFromEnvOr 物化有效配置：显式非空 URL 优先，否则按序查 env
// （HTTPS_PROXY / https_proxy / HTTP_PROXY / http_proxy），仍可能 HTTPSProxy="" 表示直连。
// 对齐 proxy.rs:45-65。
func ProxyConfigFromEnvOr(explicit *ProxyConfig) ProxyConfig {
	if explicit != nil && strings.TrimSpace(explicit.HTTPSProxy) != "" {
		return *explicit
	}
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if v := os.Getenv(k); v != "" {
			return ProxyConfig{HTTPSProxy: v}
		}
	}
	return ProxyConfig{}
}

// HTTPClient 按本配置造一个独立 *http.Client（30s 总超时 + 10s 连接，对齐 build_reqwest）。
// 代理非法返错。UA 由调用方按请求设（见 probeUserAgent）。
func (c ProxyConfig) HTTPClient() (*http.Client, error) {
	return httpx.NewClient(httpx.ClientOptions{
		Proxy:          c.HTTPSProxy,
		ProxyAuth:      c.Auth,
		Timeout:        30 * time.Second,
		ConnectTimeout: 10 * time.Second,
	})
}

// Probe 发 5s HEAD https://github.com/。ok = 2xx || 3xx。对齐 proxy.rs:92-109。
func (c ProxyConfig) Probe(ctx context.Context) (ProbeResult, error) {
	client, err := c.HTTPClient()
	if err != nil {
		return ProbeResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://github.com/", nil)
	if err != nil {
		return ProbeResult{}, err
	}
	req.Header.Set("User-Agent", probeUserAgent)
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return ProbeResult{}, err
	}
	defer resp.Body.Close()
	return ProbeResult{
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 400,
		StatusCode: resp.StatusCode,
		LatencyMs:  time.Since(start).Milliseconds(),
	}, nil
}
