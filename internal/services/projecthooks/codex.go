package projecthooks

import (
	"fmt"
	"os"
	"path/filepath"

	"xkpane/internal/utils/filex"

	toml "github.com/pelletier/go-toml/v2"
)

// syncCodexHooks 把 codexHookDefs 注册进 <proj>/.codex/hooks.json，并在 <proj>/.codex/config.toml
// 写 [features] hooks=true（codex hook 特性开关，需此开关才会触发）。移植 cc-pane codex.rs
// sync_project_hooks（D6d-5）。
//
// 注：feature flag 键为 `hooks`（手测发现 cc-pane 当年用的 `codex_hooks` 已被新版 codex 弃用，
// 提示「Use [features].hooks instead」）→ 改用新键。
//
// 偏离 cc-pane：cc-pane 在 Windows 上把 codex project hooks 标记为不支持；XK 按用户拍板照常写入，
// Windows 能否真触发由 XK 自行手测确认（见 P6D_PLAN §6 风险）。仅保留「.codex 是文件」的冲突检查。
func syncCodexHooks(projectPath, hookBinary string) error {
	codexDir := filepath.Join(projectPath, ".codex")
	if fi, err := os.Stat(codexDir); err == nil && !fi.IsDir() {
		return fmt.Errorf("项目根存在名为 .codex 的文件，无法配置 codex hook")
	}

	if err := ensureCodexHooksFeature(projectPath); err != nil {
		return err
	}

	hooksPath := filepath.Join(codexDir, "hooks.json")
	root, err := readJSONObject(hooksPath)
	if err != nil {
		return err
	}
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

	return writeJSONObject(hooksPath, root)
}

// buildCodexHookEntry 与 claude 版同形，但用 codex 的 statusMessage 字段（无 async）。
func buildCodexHookEntry(hookBinary string, def hookDef) map[string]any {
	command := fmt.Sprintf("%q %s", hookBinary, def.subcommand)
	return map[string]any{
		"matcher": def.matcher,
		"hooks": []any{
			map[string]any{
				"type":          "command",
				"command":       command,
				"timeout":       def.timeout,
				"statusMessage": "Loading XK-Pane context",
			},
		},
	}
}

// ensureCodexHooksFeature 读 <proj>/.codex/config.toml → 设 [features] hooks=true → 写回。
// 保留用户其余配置（与 cc-pane 同：整文件 round-trip，注释/格式不保真——toml 库重新序列化）。
func ensureCodexHooksFeature(projectPath string) error {
	configPath := filepath.Join(projectPath, ".codex", "config.toml")

	root := map[string]any{}
	if data, err := os.ReadFile(configPath); err == nil && len(data) > 0 {
		if err := toml.Unmarshal(data, &root); err != nil {
			// 损坏配置：回退空表（不因坏配置阻断启动，对齐 claude 侧容错）。
			root = map[string]any{}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	features, ok := root["features"].(map[string]any)
	if !ok {
		features = map[string]any{}
		root["features"] = features
	}
	features["hooks"] = true
	delete(features, "codex_hooks") // 清掉旧键，避免新版 codex 的弃用警告

	out, err := toml.Marshal(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	return filex.WriteAtomic(configPath, out, 0o644)
}
