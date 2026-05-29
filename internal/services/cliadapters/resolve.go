package cliadapters

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"xkpane/internal/utils/logx"
)

// resolveSimple 走标准 PATH 解析（which）；找不到返回带工具名的错误。
// 多数 adapter（codex/gemini/kimi/glm/cursor/opencode）用它。
func resolveSimple(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s CLI not found in PATH", name)
	}
	return path, nil
}

// resolveClaudeLike 解析 claude / reclaude：先 PATH，Windows 再回退到常见 npm/scoop/winget 目录。
// 对应 cc-pane resolve_claude_path / resolve_reclaude_path。
func resolveClaudeLike(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		if path := findWindowsExecutable(name); path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s CLI not found in PATH", name)
}

// findWindowsExecutable 在 Windows 常见安装目录 + PATH 中查找可执行（带 PATHEXT 扩展名探测）。
// 仅在 runtime.GOOS == "windows" 时由 resolveClaudeLike 调用；其余平台 os.Getenv 返回 "" 自然跳过。
func findWindowsExecutable(name string) string {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "AppData", "Roaming", "npm"),
			filepath.Join(home, "scoop", "shims"),
		)
	}
	if v := os.Getenv("APPDATA"); v != "" {
		dirs = append(dirs, filepath.Join(v, "npm"))
	}
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		dirs = append(dirs, filepath.Join(v, "Microsoft", "WinGet", "Links"))
	}
	if v := os.Getenv("SCOOP"); v != "" {
		dirs = append(dirs, filepath.Join(v, "shims"))
	}
	if v := os.Getenv("PATH"); v != "" {
		dirs = append(dirs, filepath.SplitList(v)...)
	}
	return findExecutableInDirs(name, dirs, windowsExecutableExtensions())
}

// windowsExecutableExtensions 返回探测顺序：先 PATHEXT（规范化为小写 .ext），再补默认 .exe/.cmd/.bat/.com，去重。
func windowsExecutableExtensions() []string {
	var ordered []string
	seen := map[string]bool{}
	add := func(ext string) {
		if ext != "" && !seen[ext] {
			seen[ext] = true
			ordered = append(ordered, ext)
		}
	}
	if v := os.Getenv("PATHEXT"); v != "" {
		for _, raw := range strings.Split(v, ";") {
			e := strings.TrimSpace(raw)
			if e == "" {
				continue
			}
			e = strings.ToLower(e)
			if !strings.HasPrefix(e, ".") {
				e = "." + e
			}
			add(e)
		}
	}
	for _, e := range []string{".exe", ".cmd", ".bat", ".com"} {
		add(e)
	}
	return ordered
}

// findExecutableInDirs 在给定目录里查找可执行：先尝试直接命中（含已带扩展名的情况），
// 否则按 extensions 逐个拼后缀；跳过空目录、非目录与重复目录（大小写不敏感去重）。
// 对应 cc-pane find_executable_in_dirs（Rust 标了 #[cfg(any(windows,test))]，此处纯函数跨平台可测）。
func findExecutableInDirs(executable string, dirs []string, extensions []string) string {
	hasExt := filepath.Ext(executable) != ""
	var seen []string

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			continue
		}
		normalized := strings.ToLower(dir)
		dup := false
		for _, s := range seen {
			if s == normalized {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		seen = append(seen, normalized)

		direct := filepath.Join(dir, executable)
		if isFile(direct) {
			return direct
		}
		if hasExt {
			continue
		}
		for _, ext := range extensions {
			candidate := filepath.Join(dir, executable+ext)
			if isFile(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// isWindowsBatchFile 判断路径是否 .cmd/.bat（npm 在 Windows 安装的 shim）。
func isWindowsBatchFile(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	return ext == "cmd" || ext == "bat"
}

// claudeCliJsForNpmShim 由 shim 同级目录推 node_modules/@anthropic-ai/claude-code/cli.js。
// claude 与 reclaude 共享同一 npm 包（@anthropic-ai/claude-code）。
func claudeCliJsForNpmShim(shimPath string) string {
	dir := filepath.Dir(shimPath)
	cliJS := filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-code", "cli.js")
	if isFile(cliJS) {
		return cliJS
	}
	return ""
}

// nodeForNpmShim 优先用 shim 同级的 node.exe，否则回退 PATH 中的 node。
func nodeForNpmShim(shimPath string) string {
	dir := filepath.Dir(shimPath)
	adjacent := filepath.Join(dir, "node.exe")
	if isFile(adjacent) {
		return adjacent
	}
	if p, err := exec.LookPath("node"); err == nil {
		return p
	}
	return ""
}

// windowsNpmShimInvocation 把 npm 的 .cmd/.bat shim 调用改写为 `node cli.js ...`，规避在
// Windows 上经 cmd shim 启动时的参数转义 / 控制台问题。非 batch 文件原样返回。
// 解析不到 node/cli.js 时记一条 warn 并回退直接调 shim。
// 对应 cc-pane windows_npm_shim_invocation（Rust #[cfg(any(windows,test))]）。
func windowsNpmShimInvocation(path string, args []string) (string, []string) {
	if !isWindowsBatchFile(path) {
		return path, args
	}
	node := nodeForNpmShim(path)
	cliJS := claudeCliJsForNpmShim(path)
	if node != "" && cliJS != "" {
		effective := append([]string{cliJS}, args...)
		return node, effective
	}
	logx.For("cliadapters").Warn(
		"Windows npm shim detected but node/cli.js could not be resolved; launching shim directly",
		"command", path,
	)
	return path, args
}
