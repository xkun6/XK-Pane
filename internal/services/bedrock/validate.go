package bedrock

import (
	"encoding/json"
	"fmt"
	"strings"
)

// validate.go：Phase 5 校验器共享报告骨架（对齐 validate/mod.rs）+ validate_test（纯 JSON 字符串，
// 零文件系统，对齐 validate/test_schema.rs）。json_defs / assets 在 validate_jsondefs.go / validate_assets.go。
//
// 校验器一律返回报告字符串、**不返 Err**（路径错误也是报告字符串）。Severity 用 Error/Warning/Info
// （与 review 的 Critical/Warning/Suggestion 是两套，故单独命名 issueSeverity）。

type issueSeverity int

const (
	issueError issueSeverity = iota
	issueWarning
	issueInfo
)

type issue struct {
	severity issueSeverity
	location string // 相对校验根的路径，或 "(scan)" 等合成标签
	message  string
}

// renderValidateReport 渲染 json_defs / assets 共享的 Markdown 报告。对齐 mod.rs:65-99。
func renderValidateReport(title string, scanned int, issues []issue) string {
	var e, w, i int
	for _, it := range issues {
		switch it.severity {
		case issueError:
			e++
		case issueWarning:
			w++
		case issueInfo:
			i++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", title)
	b.WriteByte('\n')
	fmt.Fprintf(&b, "Files scanned: %d\n", scanned)
	fmt.Fprintf(&b, "Issues: %d (Errors: %d, Warnings: %d, Infos: %d)\n", len(issues), e, w, i)
	b.WriteByte('\n')

	if len(issues) == 0 {
		b.WriteString("## All checks passed\n")
		return b.String()
	}
	renderValidateSection(&b, "Errors", issueError, issues)
	renderValidateSection(&b, "Warnings", issueWarning, issues)
	renderValidateSection(&b, "Infos", issueInfo, issues)
	return b.String()
}

func renderValidateSection(b *strings.Builder, title string, sev issueSeverity, all []issue) {
	var bucket []issue
	for _, it := range all {
		if it.severity == sev {
			bucket = append(bucket, it)
		}
	}
	if len(bucket) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n", title)
	b.WriteByte('\n')
	for _, it := range bucket {
		fmt.Fprintf(b, "- `%s`: %s\n", it.location, it.message)
	}
	b.WriteByte('\n')
}

// ── validate_test（test_schema.rs）─────────────────────────────────────

// validTestStepTypes：合法 step type。debug 文案 validTestTypesDebug 复刻 Rust `{:?}`。
var validTestStepTypes = []string{"wait", "assert", "action", "screenshot"}

const validTestTypesDebug = `["wait", "assert", "action", "screenshot"]`

// validateTest ← pe_validate_test。纯 JSON 字符串校验，返回单行 VALID 或多行 INVALID。
// 对齐 test_schema.rs:27-81。
func validateTest(testJSON string) (string, error) {
	var value any
	if err := json.Unmarshal([]byte(testJSON), &value); err != nil {
		return fmt.Sprintf("INVALID: JSON parse error: %v", err), nil
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return "INVALID: 1 issues:\n- top-level value must be a JSON object", nil
	}

	var issues []string

	// name
	if v, present := obj["name"]; !present {
		issues = append(issues, "name: required field missing")
	} else if s, isStr := v.(string); isStr {
		if s == "" {
			issues = append(issues, "name: must be a non-empty string")
		}
	} else {
		issues = append(issues, "name: must be a string")
	}

	// steps
	stepsCount := 0
	if v, present := obj["steps"]; !present {
		issues = append(issues, "steps: required field missing")
	} else if arr, isArr := v.([]any); isArr {
		stepsCount = len(arr)
		for i, step := range arr {
			checkTestStep(i, step, &issues)
		}
	} else {
		issues = append(issues, "steps: must be an array")
	}

	if len(issues) == 0 {
		return fmt.Sprintf("VALID: %d steps, all checks passed", stepsCount), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "INVALID: %d issues:", len(issues))
	for _, it := range issues {
		b.WriteString("\n- ")
		b.WriteString(it)
	}
	return b.String(), nil
}

func checkTestStep(i int, step any, issues *[]string) {
	obj, ok := step.(map[string]any)
	if !ok {
		*issues = append(*issues, fmt.Sprintf("step %d: must be an object", i))
		return
	}

	tv, present := obj["type"]
	if !present {
		*issues = append(*issues, fmt.Sprintf("step %d: missing \"type\" field", i))
		return
	}
	ty, isStr := tv.(string)
	if !isStr {
		*issues = append(*issues, fmt.Sprintf("step %d: \"type\" must be a string", i))
		return
	}
	if !containsStr(validTestStepTypes, ty) {
		*issues = append(*issues, fmt.Sprintf("step %d: unknown type \"%s\"; expected one of %s", i, ty, validTestTypesDebug))
		return
	}

	switch ty {
	case "wait":
		if v, present := obj["seconds"]; !present {
			*issues = append(*issues, fmt.Sprintf("step %d (wait): missing \"seconds\" field", i))
		} else if !isJSONNumber(v) {
			*issues = append(*issues, fmt.Sprintf("step %d (wait): \"seconds\" must be a number", i))
		}
	case "assert":
		if v, present := obj["expr"]; !present {
			*issues = append(*issues, fmt.Sprintf("step %d (assert): missing \"expr\" field", i))
		} else if s, isStr := v.(string); isStr {
			if s == "" {
				*issues = append(*issues, fmt.Sprintf("step %d (assert): \"expr\" must be a non-empty string", i))
			}
		} else {
			*issues = append(*issues, fmt.Sprintf("step %d (assert): \"expr\" must be a string", i))
		}
	case "action":
		if v, present := obj["command"]; !present {
			*issues = append(*issues, fmt.Sprintf("step %d (action): missing \"command\" field", i))
		} else if _, isStr := v.(string); !isStr {
			*issues = append(*issues, fmt.Sprintf("step %d (action): \"command\" must be a string", i))
		}
	case "screenshot":
		// region 可选，无额外检查。
	}
}

// isJSONNumber：encoding/json 把 JSON 数字解成 float64（对齐 serde Value::is_number）。
func isJSONNumber(v any) bool {
	_, ok := v.(float64)
	return ok
}
