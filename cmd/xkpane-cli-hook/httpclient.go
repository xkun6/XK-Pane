package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// hookTimeout 是所有回连请求的硬超时：750ms 内必须完成，否则放弃（绝不阻塞 CLI）。
const hookTimeout = 750 * time.Millisecond

func hookHTTPClient() *http.Client {
	return &http.Client{Timeout: hookTimeout}
}

// resolveBaseURL 解析 orchestrator REST 基址：优先 XKPANE_API_BASE_URL，回退
// http://127.0.0.1:${XKPANE_API_PORT}。两者皆无则返回 ""（调用方静默放弃）。
// 对齐 cc-pane session_start.rs send_session_started 的 env 解析顺序。
func resolveBaseURL() string {
	if u := strings.TrimSpace(os.Getenv("XKPANE_API_BASE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	if p := strings.TrimSpace(os.Getenv("XKPANE_API_PORT")); p != "" {
		return "http://127.0.0.1:" + p
	}
	return ""
}

func apiToken() string {
	return strings.TrimSpace(os.Getenv("XKPANE_API_TOKEN"))
}

// postJSON 向 url POST JSON body（带 Bearer token），返回状态码、响应体（截断 512 字节）与错误。
// 错误处理由调用方决定（session-start 静默；notify 上报）。
func postJSON(url, token string, body []byte) (int, string, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := hookHTTPClient().Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	// 上限放宽到 32KB：session-start 响应内联 workspaceProjects 清单需完整解析，512 字节会
	// 截断 JSON。仍设上限防异常大响应撑爆一次性进程。
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
	return resp.StatusCode, string(respBody), nil
}
