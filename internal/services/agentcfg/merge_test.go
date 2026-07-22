package agentcfg

import (
	"encoding/json"
	"testing"
)

// mustJSONMap 把 JSON 字面量字符串解析成 map[string]any,供测试构造输入(数字统一为
// float64,与 deepMergeJSON/deepDiffJSON 等纯函数处理 json.Unmarshal 产物的场景一致)。
func mustJSONMap(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("解析 JSON 失败: %v\n输入: %s", err, s)
	}
	return m
}

func TestDeepMergeJSON_Basic(t *testing.T) {
	base := mustJSONMap(t, `{"a":1,"env":{"X":1,"Y":2}}`)
	patch := mustJSONMap(t, `{"b":2,"env":{"Y":9,"Z":3}}`)
	baseCopy := mustJSONMap(t, `{"a":1,"env":{"X":1,"Y":2}}`)
	patchCopy := mustJSONMap(t, `{"b":2,"env":{"Y":9,"Z":3}}`)

	got := deepMergeJSON(base, patch)
	want := mustJSONMap(t, `{"a":1,"b":2,"env":{"X":1,"Y":9,"Z":3}}`)

	if !jsonEqual(got, want) {
		t.Fatalf("deepMergeJSON 结果不符:\n got  = %v\n want = %v", got, want)
	}
	if !jsonEqual(base, baseCopy) {
		t.Fatalf("deepMergeJSON 修改了入参 base: %v", base)
	}
	if !jsonEqual(patch, patchCopy) {
		t.Fatalf("deepMergeJSON 修改了入参 patch: %v", patch)
	}
}

func TestDeepMergeJSON_ArrayReplacedNotConcatenated(t *testing.T) {
	base := mustJSONMap(t, `{"arr":[1,2]}`)
	patch := mustJSONMap(t, `{"arr":[3]}`)

	got := deepMergeJSON(base, patch)
	want := mustJSONMap(t, `{"arr":[3]}`)

	if !jsonEqual(got, want) {
		t.Fatalf("deepMergeJSON 数组应整体替换而非拼接:\n got  = %v\n want = %v", got, want)
	}
}

func TestDeepMergeJSON_TypeMismatchOverride(t *testing.T) {
	base := mustJSONMap(t, `{"a":{"x":1}}`)
	patch := mustJSONMap(t, `{"a":5}`)

	got := deepMergeJSON(base, patch)
	want := mustJSONMap(t, `{"a":5}`)

	if !jsonEqual(got, want) {
		t.Fatalf("deepMergeJSON 类型不一致时应整体覆盖:\n got  = %v\n want = %v", got, want)
	}
}

func TestDeepDiffJSON_Basic(t *testing.T) {
	sub := mustJSONMap(t, `{"a":1,"env":{"X":1,"Y":9}}`)
	base := mustJSONMap(t, `{"a":1,"env":{"X":1,"Y":2}}`)

	got := deepDiffJSON(sub, base)
	want := mustJSONMap(t, `{"env":{"Y":9}}`)

	if !jsonEqual(got, want) {
		t.Fatalf("deepDiffJSON 结果不符:\n got  = %v\n want = %v", got, want)
	}
}

func TestDeepDiffJSON_EqualYieldsEmpty(t *testing.T) {
	sub := mustJSONMap(t, `{"a":1,"env":{"X":1,"Y":2}}`)
	base := mustJSONMap(t, `{"a":1,"env":{"X":1,"Y":2}}`)

	got := deepDiffJSON(sub, base)
	if len(got) != 0 {
		t.Fatalf("sub 与 base 相同时 diff 应为空,got = %v", got)
	}
}

func TestDeepDiffJSON_MergeRoundTrip(t *testing.T) {
	base := mustJSONMap(t, `{"a":1,"env":{"X":1,"Y":2},"model":"m1"}`)
	// sub 包含 base 的每个键,部分值不同(env.Y、model),且新增一个 base 没有的键(env.Z)。
	sub := mustJSONMap(t, `{"a":1,"env":{"X":1,"Y":9,"Z":3},"model":"m2"}`)

	diff := deepDiffJSON(sub, base)
	merged := deepMergeJSON(base, diff)

	if !jsonEqual(merged, sub) {
		t.Fatalf("deepMergeJSON(base, deepDiffJSON(sub, base)) 应语义等价于 sub:\n merged = %v\n sub    = %v", merged, sub)
	}
}

func TestSplitByPaths_RoundTrip(t *testing.T) {
	live := mustJSONMap(t, `{"a":1,"env":{"TOKEN":"x","DISABLE":"1"},"model":"m"}`)
	pathset := map[string]struct{}{
		"env" + leafPathSep + "TOKEN": {},
		"model":                       {},
	}

	patchPart, basePart := splitByPaths(live, pathset)

	wantPatch := mustJSONMap(t, `{"env":{"TOKEN":"x"},"model":"m"}`)
	wantBase := mustJSONMap(t, `{"a":1,"env":{"DISABLE":"1"}}`)

	if !jsonEqual(patchPart, wantPatch) {
		t.Fatalf("patchPart 不符:\n got  = %v\n want = %v", patchPart, wantPatch)
	}
	if !jsonEqual(basePart, wantBase) {
		t.Fatalf("basePart 不符:\n got  = %v\n want = %v", basePart, wantBase)
	}

	merged := deepMergeJSON(basePart, patchPart)
	if !jsonEqual(merged, live) {
		t.Fatalf("deepMergeJSON(basePart, patchPart) 应语义等价于 live:\n merged = %v\n live   = %v", merged, live)
	}
}

func TestSplitByPaths_EmptyObjectLeafRoundTrip(t *testing.T) {
	live := mustJSONMap(t, `{"permissions":{}}`)
	pathset := map[string]struct{}{}

	patchPart, basePart := splitByPaths(live, pathset)

	wantBase := mustJSONMap(t, `{"permissions":{}}`)
	wantPatch := map[string]any{}

	if !jsonEqual(basePart, wantBase) {
		t.Fatalf("basePart 不符:\n got  = %v\n want = %v", basePart, wantBase)
	}
	if !jsonEqual(patchPart, wantPatch) {
		t.Fatalf("patchPart 应为空:\n got  = %v\n want = %v", patchPart, wantPatch)
	}

	merged := deepMergeJSON(basePart, patchPart)
	if !jsonEqual(merged, live) {
		t.Fatalf("deepMergeJSON(basePart, patchPart) 应语义等价于 live:\n merged = %v\n live   = %v", merged, live)
	}
}
