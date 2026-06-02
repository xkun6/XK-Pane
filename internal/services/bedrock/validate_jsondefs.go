package bedrock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// validate_jsondefs.go：pe_validate_json_definitions。扫 addon 行为包根下 4 个子目录的 .json，
// 逐文件 C1-C5 检查。对齐 validate/json_defs.rs。

// scanDir：(子目录, 期望 format_version)。"" = 仅做字符串检查不校验具体值。
type scanDir struct {
	subdir   string
	expected string
}

var jsonScanDirs = []scanDir{
	{"netease_items_beh", "1.10"},
	{"netease_blocks", "1.10.0"},
	{"netease_recipes", ""},
	{"entities", ""},
}

// validateJSONDefinitions ← pe_validate_json_definitions。返回 Markdown 报告（路径错误也是报告）。
func validateJSONDefinitions(addonDir string) (string, error) {
	if msg, bad := checkAddonPath(addonDir, "JSON Definitions Validation"); bad {
		return msg, nil
	}

	var issues []issue
	scanned := 0
	for _, sd := range jsonScanDirs {
		sub := filepath.Join(addonDir, sd.subdir)
		if fi, err := os.Stat(sub); err != nil || !fi.IsDir() {
			issues = append(issues, issue{issueInfo, fmt.Sprintf("(%s)", sd.subdir), "subdirectory not present, skipped"})
			continue
		}
		var files []string
		collectJSON(sub, &files)
		for _, f := range files {
			scanned++
			checkJSONFile(addonDir, f, sd.subdir, sd.expected, &issues)
		}
	}
	return renderValidateReport(fmt.Sprintf("JSON Definitions Validation: %s", addonDir), scanned, issues), nil
}

// checkAddonPath 复刻路径三段校验（绝对/存在/目录），返回 (报告字符串, 是否非法)。
// 对齐 json_defs.rs:33-47 / assets.rs:48-62（title 不同）。
func checkAddonPath(addonDir, title string) (string, bool) {
	if !filepath.IsAbs(addonDir) {
		return fmt.Sprintf("# %s: %s\n\nError: addon_dir must be an absolute path\n", title, addonDir), true
	}
	fi, err := os.Stat(addonDir)
	if err != nil {
		return fmt.Sprintf("# %s: %s\n\nError: addon_dir does not exist\n", title, addonDir), true
	}
	if !fi.IsDir() {
		return fmt.Sprintf("# %s: %s\n\nError: addon_dir is not a directory\n", title, addonDir), true
	}
	return "", false
}

// collectJSON 递归收集扩展名严格为 "json"（**大小写敏感**，与 assets 不同）的文件。对齐 json_defs.rs:76-88。
func collectJSON(dir string, out *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			collectJSON(p, out)
		} else if extOf(e.Name()) == "json" {
			*out = append(*out, p)
		}
	}
}

// extOf 取文件名最后一段扩展名（大小写保留），无则 ""。复刻 Rust Path::extension（前导点的 dotfile 无扩展）。
func extOf(name string) string {
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 {
		return ""
	}
	return name[dot+1:]
}

func relativePath(root, file string) string {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return filepath.ToSlash(file)
	}
	return filepath.ToSlash(rel)
}

func checkJSONFile(root, file, subdir, expectedVer string, issues *[]issue) {
	rel := relativePath(root, file)
	raw, err := os.ReadFile(file)
	if err != nil {
		*issues = append(*issues, issue{issueError, rel, fmt.Sprintf("cannot read file: %v", err)})
		return
	}

	// C2 — BOM
	hasBOM := len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF
	if hasBOM {
		*issues = append(*issues, issue{issueWarning, rel, "file starts with UTF-8 BOM"})
	}

	// C1 — UTF-8
	stripped := raw
	if hasBOM {
		stripped = raw[3:]
	}
	if !utf8.Valid(stripped) {
		*issues = append(*issues, issue{issueError, rel, "not valid UTF-8: invalid UTF-8 sequence"})
		return
	}
	text := string(stripped)

	// C3 — parse
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		line, col := jsonLineCol(text, err)
		*issues = append(*issues, issue{issueError, rel, fmt.Sprintf("JSON parse error at line %d column %d: %v", line, col, err)})
		return
	}

	// C4 — format_version
	obj, ok := value.(map[string]any)
	if !ok {
		*issues = append(*issues, issue{issueWarning, rel, "top-level JSON is not an object"})
		return
	}
	if fv, present := obj["format_version"]; !present {
		*issues = append(*issues, issue{issueWarning, rel, "missing top-level \"format_version\" field"})
	} else if s, isStr := fv.(string); isStr {
		if expectedVer != "" && s != expectedVer {
			*issues = append(*issues, issue{issueWarning, rel, fmt.Sprintf("format_version is \"%s\"; netease %s expects \"%s\"", s, subdir, expectedVer)})
		}
	} else {
		*issues = append(*issues, issue{issueWarning, rel, "format_version exists but is not a string"})
	}

	// C5 — namespace
	if desc, ok := findDescription(obj); ok {
		checkIdentifier(rel, desc, issues)
	}
}

// findDescription：先 $.description，否则在子对象里找 $.<child>.description。子对象按键升序遍历
// （对齐 serde_json::Map 默认 BTreeMap 的有序迭代，且让 Go map 无序下结果确定）。对齐 json_defs.rs:184-198。
func findDescription(obj map[string]any) (map[string]any, bool) {
	if d, ok := obj["description"].(map[string]any); ok {
		return d, true
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if child, ok := obj[k].(map[string]any); ok {
			if d, ok := child["description"].(map[string]any); ok {
				return d, true
			}
		}
	}
	return nil, false
}

func checkIdentifier(rel string, desc map[string]any, issues *[]issue) {
	idv, present := desc["identifier"]
	if !present {
		*issues = append(*issues, issue{issueInfo, rel, "description.identifier not found"})
		return
	}
	id, isStr := idv.(string)
	if !isStr {
		*issues = append(*issues, issue{issueWarning, rel, "description.identifier is present but not a string"})
		return
	}
	colon := strings.IndexByte(id, ':')
	if colon < 0 {
		*issues = append(*issues, issue{issueWarning, rel, fmt.Sprintf("identifier \"%s\" missing namespace prefix", id)})
		return
	}
	switch id[:colon] {
	case "minecraft":
		*issues = append(*issues, issue{issueInfo, rel, "identifier uses \"minecraft:\" namespace (override is allowed but discouraged)"})
	case "":
		*issues = append(*issues, issue{issueWarning, rel, fmt.Sprintf("identifier \"%s\" has empty namespace", id)})
	}
}

// jsonLineCol 从 Go json 错误（SyntaxError.Offset）自算 1-based 行列复刻 serde 报错文案。
// 非 SyntaxError（理论上 Unmarshal 到 any 只产 SyntaxError）退化为 (1,1)。
func jsonLineCol(text string, err error) (int, int) {
	off := -1
	if se, ok := err.(*json.SyntaxError); ok {
		off = int(se.Offset)
	}
	if off < 0 {
		return 1, 1
	}
	if off > len(text) {
		off = len(text)
	}
	line, col := 1, 1
	for i := 0; i < off; i++ {
		if text[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}
