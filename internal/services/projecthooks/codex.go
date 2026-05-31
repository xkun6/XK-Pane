package projecthooks

import (
	"fmt"
	"os"
	"path/filepath"

	"xkpane/internal/utils/filex"

	toml "github.com/pelletier/go-toml/v2"
)

// syncCodexHooks 把 codexHookDefs【内联】注册进 <proj>/.codex/config.toml 的 [[hooks.SessionStart]]，
// 并写 [features] hooks=true。移植 cc-pane codex.rs sync_project_hooks 思路（D6d-5），但落点据手测调整。
//
// 落点改为内联 config.toml（而非独立 hooks.json）：手测发现新版 codex 读了项目 config.toml（弃用
// 警告为证）却未触发独立 hooks.json 里的 hook。官方文档明确 codex「discovers hooks ... [as]
// hooks.json [or] inline [hooks] tables inside config.toml」，二者同 schema。改写进【已证实被 codex
// 读取】的 config.toml，命中率最高；并清理旧的独立 hooks.json 避免双源重复。
//
// feature flag 键为 hooks（cc-pane 当年的 codex_hooks 已被新版 codex 弃用）。
//
// 偏离 cc-pane：cc-pane 在 Windows 把 codex project hooks 标记为不支持；XK 按用户拍板照常写入，
// Windows 能否真触发由手测确认。仅保留「.codex 是文件」的冲突检查。
func syncCodexHooks(projectPath, hookBinary string) error {
	codexDir := filepath.Join(projectPath, ".codex")
	if fi, err := os.Stat(codexDir); err == nil && !fi.IsDir() {
		return fmt.Errorf("项目根存在名为 .codex 的文件，无法配置 codex hook")
	}

	configPath := filepath.Join(codexDir, "config.toml")
	root, err := readTOMLObject(configPath)
	if err != nil {
		return err
	}

	// [features] hooks=true（清掉旧 codex_hooks 键避免弃用警告）。
	features, ok := root["features"].(map[string]any)
	if !ok {
		features = map[string]any{}
		root["features"] = features
	}
	features["hooks"] = true
	delete(features, "codex_hooks")

	// 内联 [[hooks.<Event>]]：复用 claude 侧的 merge/dedup（保留用户其它 hook 事件与外部条目）。
	hooks := ensureObject(root, "hooks")
	for _, def := range codexHookDefs {
		entry := buildCodexHookEntry(hookBinary, def)
		mergeManagedEntry(hooks, def.event, entry)
	}
	for event, v := range hooks {
		if arr, ok := v.([]any); ok && len(arr) == 0 {
			delete(hooks, event)
		}
	}

	if err := writeTOMLObject(configPath, root); err != nil {
		return err
	}
	// 清理旧的独立 hooks.json（之前版本写过；codex 未认它，留着只会造成双源/困惑）。
	_ = os.Remove(filepath.Join(codexDir, "hooks.json"))
	return nil
}

// buildCodexHookEntry 与 claude 版同形，但用 codex 的 statusMessage 字段（无 async），并补
// command_windows：codex 的通用 command 在 Windows 上经 sh/bash 执行，bash 会把 C:\path 的反斜杠
// 当转义吃掉 → 路径损坏 → exit 1（手测 codex 报 "hook exited with code 1" 且未注入即此因）。
// command_windows 经 cmd.exe 执行，反斜杠原样保留（官方 Windows 覆盖字段）。
func buildCodexHookEntry(hookBinary string, def hookDef) map[string]any {
	command := hookCommand(hookBinary, def.subcommand)
	return map[string]any{
		"matcher": def.matcher,
		"hooks": []any{
			map[string]any{
				"type":            "command",
				"command":         command,
				"command_windows": command,
				"timeout":         def.timeout,
				"statusMessage":   "Loading XK-Pane context",
			},
		},
	}
}

// readTOMLObject 读 config.toml 为 map；不存在返回空表；损坏回退空表（不因坏配置阻断启动）。
func readTOMLObject(path string) (map[string]any, error) {
	root := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return root, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return root, nil
	}
	if err := toml.Unmarshal(data, &root); err != nil {
		return map[string]any{}, nil
	}
	if root == nil {
		root = map[string]any{}
	}
	return root, nil
}

// writeTOMLObject 以 go-toml/v2 序列化并原子写（整文件 round-trip，注释/格式不保真）。
func writeTOMLObject(path string, root map[string]any) error {
	out, err := toml.Marshal(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return filex.WriteAtomic(path, out, 0o644)
}
