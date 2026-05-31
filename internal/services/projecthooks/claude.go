package projecthooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xkpane/internal/utils/filex"
)

// syncClaudeHooks 把 claudeHookDefs 注册进 <proj>/.claude/settings.local.json 的 hooks 块。
// 忠实移植 cc-pane claude.rs sync_project_hooks：同事件先剔除本工具旧条目再追加（幂等，重启不堆叠），
// 保留其它工具/用户的 hook 条目不动；空事件数组清理；filex 原子写（配置写铁律）。
func syncClaudeHooks(projectPath, hookBinary string) error {
	settingsPath := filepath.Join(projectPath, ".claude", "settings.local.json")
	settings, err := readJSONObject(settingsPath)
	if err != nil {
		return err
	}

	hooks := ensureObject(settings, "hooks")

	for _, def := range claudeHookDefs {
		entry := buildHookEntry(hookBinary, def)
		mergeManagedEntry(hooks, def.event, entry)
	}

	// 清理空事件数组（对齐 cc-pane retain）。
	for event, v := range hooks {
		if arr, ok := v.([]any); ok && len(arr) == 0 {
			delete(hooks, event)
		}
	}

	return writeJSONObject(settingsPath, settings)
}

// buildHookEntry 构造单条 hook 配置对象：
// { matcher, hooks: [{ type:"command", command:"\"<bin>\" <subcommand>", timeout, async:true }] }
// 注：command 里路径不预转义反斜杠——json.Marshal 会正确转义为单层 \\（运行时还原为单反斜杠的合法
// Windows 路径），比 cc-pane 的双重转义更干净。
func buildHookEntry(hookBinary string, def hookDef) map[string]any {
	command := fmt.Sprintf("%q %s", hookBinary, def.subcommand)
	return map[string]any{
		"matcher": def.matcher,
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
				"timeout": def.timeout,
				"async":   true,
			},
		},
	}
}

// mergeManagedEntry 把 entry 追加进 hooks[event] 数组，先剔除本工具托管的旧条目（按 command 含
// hookBinaryBaseName 判定）。其它条目保留。
func mergeManagedEntry(hooks map[string]any, event string, entry map[string]any) {
	var arr []any
	if existing, ok := hooks[event].([]any); ok {
		for _, e := range existing {
			if !isManagedEntry(e) {
				arr = append(arr, e)
			}
		}
	}
	arr = append(arr, entry)
	hooks[event] = arr
}

// isManagedEntry 判定一条 hook 条目是否由本工具托管（其内任一 command 含 hookBinaryBaseName）。
func isManagedEntry(entry any) bool {
	obj, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	inner, ok := obj["hooks"].([]any)
	if !ok {
		return false
	}
	for _, h := range inner {
		ho, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := ho["command"].(string); ok && strings.Contains(cmd, hookBinaryBaseName) {
			return true
		}
	}
	return false
}

// ── 通用 JSON 对象读写（保留外部键，仅改我们的部分）─────────────────────────

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		// 文件损坏/非对象：对齐 cc-pane 回退空对象（不因坏配置阻断启动）。
		return map[string]any{}, nil
	}
	if obj == nil {
		obj = map[string]any{}
	}
	return obj, nil
}

func writeJSONObject(path string, obj map[string]any) error {
	if parent := filepath.Dir(path); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return filex.WriteAtomic(path, data, 0o644)
}

// ensureObject 返回 root[key] 的 map（不存在/类型不符则新建并写回）。
func ensureObject(root map[string]any, key string) map[string]any {
	if existing, ok := root[key].(map[string]any); ok {
		return existing
	}
	m := map[string]any{}
	root[key] = m
	return m
}
