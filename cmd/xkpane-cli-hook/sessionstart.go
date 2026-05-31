package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// sessionStartInput 是 Claude Code SessionStart hook 经 stdin 传入的 JSON 子集。
type sessionStartInput struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
}

// sessionStartedRequest 是回连 orchestrator 的上报体（camelCase，对齐 XK 前端/Wails 约定）。
// 偏离 cc-pane：去掉 launch_id（XK 按 pty_session_id 关联，D6d-1）与 wsl_distro（本轮无 WSL）。
type sessionStartedRequest struct {
	PtySessionID    string `json:"ptySessionId"`
	ResumeSessionID string `json:"resumeSessionId"`
	CliTool         string `json:"cliTool"`
	RuntimeKind     string `json:"runtimeKind"`
	Cwd             string `json:"cwd"`
}

// runSessionStart 是 SessionStart hook 入口：读 stdin → 回连上报 → stdout 注入上下文。
// 任何失败都静默吞掉（stderr 留日志），绝不让 CLI 因 hook 卡住。
func runSessionStart() {
	// 非交互模式跳过注入（对齐 cc-pane）。
	if os.Getenv("CLAUDE_NON_INTERACTIVE") == "1" {
		return
	}

	in := readSessionStartInput()
	sessionID := ""
	if in != nil {
		sessionID = in.SessionID
	}

	projectDir := resolveProjectDir(in)
	if fi, err := os.Stat(projectDir); err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "[xkpane-hook] session-start: project dir 非法: %s\n", projectDir)
		return
	}

	// 回连上报 launch_cwd / resumeId（仅在拿到 pty session id 时；失败静默）。
	if sessionID != "" {
		if err := reportSessionStarted(sessionID, projectDir); err != nil {
			fmt.Fprintf(os.Stderr, "[xkpane-hook] session-start: 上报失败: %v\n", err)
		}
	} else {
		fmt.Fprintln(os.Stderr, "[xkpane-hook] session-start: stdin 无 session_id，跳过上报")
	}

	// stdout 注入项目上下文（XK 子集：header + current-state + ready）。
	printContext(projectDir)
}

func reportSessionStarted(resumeSessionID, projectDir string) error {
	pty := strings.TrimSpace(os.Getenv("XKPANE_PTY_SESSION_ID"))
	if pty == "" {
		return fmt.Errorf("XKPANE_PTY_SESSION_ID 缺失")
	}
	base := resolveBaseURL()
	token := apiToken()
	if base == "" || token == "" {
		return fmt.Errorf("XKPANE_API_BASE_URL/PORT 或 TOKEN 缺失（orchestrator 未注入？）")
	}

	req := sessionStartedRequest{
		PtySessionID:    pty,
		ResumeSessionID: resumeSessionID,
		CliTool:         detectCliTool(),
		RuntimeKind:     detectRuntimeKind(),
		Cwd:             projectDir,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	status, snippet, err := postJSON(base+"/api/terminal/session-started", token, body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("HTTP %d: %s", status, snippet)
	}
	return nil
}

// detectCliTool 据 XKPANE_CLI_TOOL（XK 注入）判定，回退 codex env 探测，默认 claude。
func detectCliTool() string {
	switch strings.TrimSpace(os.Getenv("XKPANE_CLI_TOOL")) {
	case "codex":
		return "codex"
	case "reclaude":
		return "reclaude"
	case "claude":
		return "claude"
	}
	if os.Getenv("CODEX_HOME") != "" || os.Getenv("CODEX_SANDBOX") != "" || os.Getenv("CODEX_REMOTE") != "" {
		return "codex"
	}
	return "claude"
}

// detectRuntimeKind 本轮恒为 local（无 WSL/SSH 支持），但仍据 env 探测以备未来。
func detectRuntimeKind() string {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return "ssh"
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return "wsl"
	}
	return "local"
}

// resolveProjectDir 取 stdin.cwd → CLAUDE_PROJECT_DIR → os.Getwd，再尝试规范化绝对路径。
func resolveProjectDir(in *sessionStartInput) string {
	var dir string
	if in != nil && strings.TrimSpace(in.Cwd) != "" {
		dir = in.Cwd
	} else if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		dir = d
	} else if wd, err := os.Getwd(); err == nil {
		dir = wd
	} else {
		dir = "."
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

// printContext 往 stdout 写注入上下文（Claude 把 SessionStart hook 的 stdout 喂给模型）。
func printContext(projectDir string) {
	fmt.Print("<xkpane-context>\n" +
		"XK-Pane has injected project context for this session.\n" +
		"Please read the following information carefully.\n" +
		"</xkpane-context>\n\n")

	fmt.Println("<current-state>")
	fmt.Printf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Branch: %s\n", gitBranch(projectDir))
	fmt.Printf("Git status:\n%s\n", gitStatus(projectDir))
	fmt.Println("</current-state>")
	fmt.Println()

	fmt.Print("<ready>\n" +
		"Context loaded. Waiting for user input.\n" +
		"</ready>\n")
}

func gitBranch(dir string) string {
	out, err := runGit(dir, "branch", "--show-current")
	if err != nil {
		return "unknown"
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "HEAD detached"
	}
	return out
}

func gitStatus(dir string) string {
	out, err := runGit(dir, "status", "--short")
	if err != nil {
		return "Unable to get git status"
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "Working tree clean"
	}
	return out
}

func runGit(dir string, args ...string) (string, error) {
	cmd := noWindowCmd("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// readSessionStartInput 读 stdin 全量并解析 JSON；空/不可读/非法返回 nil（调用方容忍）。
func readSessionStartInput() *sessionStartInput {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var in sessionStartInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil
	}
	return &in
}
