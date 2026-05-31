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

// planArchiveInput 是 PostToolUse hook 经 stdin 传入的 JSON 子集。
type planArchiveInput struct {
	SessionID     string `json:"session_id"`
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
	ToolInput     struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

// runPlanArchive 是 PostToolUse(Write) hook 入口：把写入 ~/.claude/plans/ 的 plan 文件
// 归档到 <CLAUDE_PROJECT_DIR>/.xkpanes/plans/（D6d-6 落点；纯本地，不回连）。
// 归档名：<sessionID 前 8>_<yyyyMMdd_HHmmss>_<原名>，同 session+同名去重覆盖。
func runPlanArchive() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	var in planArchiveInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return
	}

	// 仅处理 Write 工具的 PostToolUse。
	if in.HookEventName != "PostToolUse" || in.ToolName != "Write" {
		return
	}
	if strings.TrimSpace(in.ToolInput.FilePath) == "" {
		return
	}

	plansDir := claudePlansDir()
	if plansDir == "" {
		return
	}

	// 判定写入文件是否落在 ~/.claude/plans/ 下（规范化后前缀比对）。
	filePath := in.ToolInput.FilePath
	canonFile := canonical(filePath)
	canonPlans := canonical(plansDir)
	if !strings.HasPrefix(canonFile, canonPlans+string(os.PathSeparator)) && canonFile != canonPlans {
		return
	}

	projectDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if projectDir == "" {
		return
	}

	targetDir := filepath.Join(projectDir, ".xkpanes", "plans")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[xkpane-hook] plan-archive: 建目录失败: %v\n", err)
		return
	}

	originalName := filepath.Base(filePath)
	prefix := sessionPrefix(in.SessionID)

	target := findExistingArchive(targetDir, prefix, originalName)
	if target == "" {
		ts := time.Now().Format("20060102_150405")
		var name string
		if prefix == "" {
			name = fmt.Sprintf("%s_%s", ts, originalName)
		} else {
			name = fmt.Sprintf("%s_%s_%s", prefix, ts, originalName)
		}
		target = filepath.Join(targetDir, name)
	}

	if err := copyFile(filePath, target); err != nil {
		fmt.Fprintf(os.Stderr, "[xkpane-hook] plan-archive: 归档失败: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[xkpane-hook] plan-archive: %s -> %s\n", filePath, target)
}

func claudePlansDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// 即使目录暂不存在也返回路径：前缀比对会自然不命中。
	return filepath.Join(home, ".claude", "plans")
}

func sessionPrefix(sessionID string) string {
	if len(sessionID) >= 8 {
		return sessionID[:8]
	}
	return sessionID
}

// findExistingArchive 找同 session 前缀 + 同原名的既有归档（去重覆盖，不堆重复）。
func findExistingArchive(targetDir, prefix, originalName string) string {
	if prefix == "" {
		return ""
	}
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return ""
	}
	suffix := "_" + originalName
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			return filepath.Join(targetDir, name)
		}
	}
	return ""
}

// canonical 尽量返回规范化绝对路径用于前缀比对；失败回退清洗后的原路径。
func canonical(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			return resolved
		}
		return abs
	}
	return filepath.Clean(p)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
