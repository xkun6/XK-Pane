package bedrock

import (
	"fmt"
	"sort"
	"strings"
)

// standards.go：standards/*.md 取数。对齐 standards/ops.rs。

// getStandard ← bedrock_get_standard。按文件 stem 精确查（可带/不带 .md），miss 则 lower 子串模糊。
// 对齐 ops.rs:11-65。
func getStandard(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name must not be empty")
	}
	idx, err := getIndices()
	if err != nil {
		return "", err
	}

	// 允许传 "modsdk-expert" 或 "modsdk-expert.md"（.md/.MD 各试，大小写敏感）。
	key := name
	if strings.HasSuffix(name, ".md") {
		key = strings.TrimSuffix(name, ".md")
	} else if strings.HasSuffix(name, ".MD") {
		key = strings.TrimSuffix(name, ".MD")
	}

	if content, ok := idx.standards[key]; ok {
		return "# " + key + "\n\n" + content, nil
	}

	// 模糊回退：lower 子串跨 stem。
	needle := strings.ToLower(name)
	var hits []string
	for k := range idx.standards {
		if strings.Contains(strings.ToLower(k), needle) {
			hits = append(hits, k)
		}
	}
	sort.Strings(hits)

	if len(hits) == 1 {
		stem := hits[0]
		return "# " + stem + "\n\n" + idx.standards[stem], nil
	}
	if len(hits) > 0 {
		return fmt.Sprintf("No standard named \"%s\". Did you mean: %s?", name, strings.Join(hits, " / ")), nil
	}
	return fmt.Sprintf("No standard named \"%s\". Available standards can be listed via bedrock_help.", name), nil
}

// fetchRequired 取固定 stem 的内嵌标准全文（不加 # 头）；缺则报错。对齐 ops.rs:67-74。
func fetchRequired(stem string) (string, error) {
	idx, err := getIndices()
	if err != nil {
		return "", err
	}
	content, ok := idx.standards[stem]
	if !ok {
		return "", fmt.Errorf("required standard '%s' is not embedded", stem)
	}
	return content, nil
}

func expertGuide() (string, error)       { return fetchRequired("modsdk-expert") }
func reviewRules() (string, error)       { return fetchRequired("netease-code-review") }
func optimizationRules() (string, error) { return fetchRequired("netease-optimization-expert") }

// jsonSpec ← bedrock_get_json_spec。netease→仅网易；modsdk→仅 ModSDK；其它/None→both 拼接（分隔符是契约）。
// 对齐 ops.rs:96-108。
func jsonSpec(contentType *string) (string, error) {
	switch {
	case contentType != nil && *contentType == "netease":
		return fetchRequired("netease-bedrock-json")
	case contentType != nil && *contentType == "modsdk":
		return fetchRequired("modsdk-json-generator")
	default:
		a, err := fetchRequired("netease-bedrock-json")
		if err != nil {
			return "", err
		}
		b, err := fetchRequired("modsdk-json-generator")
		if err != nil {
			return "", err
		}
		return a + "\n\n---\n\n" + b, nil
	}
}

// helpText ← bedrock_help。逐字照搬 cc-pane ops.rs:112-150 的静态文本（含其「13 rules」措辞 ——
// 忠实复刻 cc-pane 输出，实际注册的审查规则为 12 条，见 review 单元）。
func helpText() (string, error) {
	return bedrockHelpBody, nil
}

const bedrockHelpBody = `Bedrock Query MCP — Embedded Minecraft China Edition ModSDK Knowledge

Phase 2 — API / Event / Component
  search_api(name)                              — fuzzy search ModSDK APIs (top 20)
  get_api(name, side?)                          — Markdown card for one API
  search_event(name)                            — fuzzy search engine events
  get_event(name, side?)                        — Markdown card for one event
  validate_call(name, side)                     — assert (name, side) exists
  api_stats()                                   — totals + top categories
  list_categories()                             — alphabetical category list
  list_apis_by_category(category)               — every API under a category
  search_component(query, component_type?)      — substring search components
  get_component(component_id)                   — full component definition

Phase 3 — Guide / Template / Standard
  search_guide(keyword, source?)                — multi-token BM25 search over mcguide/mconline/wiki/ui_templates
  read_guide(path, max_lines?)                  — read an embedded markdown guide (max 2000 lines)
  match_template(tags?, keyword?)               — rank official demo templates
  get_template_info(template_id)                — template metadata card
  get_template_file(template_id, file_path)     — read a file inside a template (when embedded)
  get_standard(name)                            — fetch any standards/*.md (exact or fuzzy)
  get_expert_guide()                            — standards/modsdk-expert.md
  get_review_rules()                            — standards/netease-code-review.md
  get_optimization_rules()                      — standards/netease-optimization-expert.md
  get_json_spec(content_type?)                  — netease + modsdk JSON authoring rules
  bedrock_help()                                   — this overview

Additional tools
  review_code(code, filename?)                  — Python 2.7 static check, 13 rules
  validate_json_definitions(addon_dir)          — addon JSON sanity check
  validate_addon_assets(addon_dir)              — texture hygiene check
  validate_test(test_json)                      — declarative test schema check
  list_mod_packs()                              — group templates by mod_name

Tips:
  * Names are case-insensitive.
  * For Chinese keywords, prefer a single core term (e.g. "维度", "UI", "传送门").
  * Always call validate_call before emitting code that invokes a ModSDK method.
`
