package mcpe

import (
	"reflect"
	"testing"
)

func TestMergeEnvOverridesAndAppends(t *testing.T) {
	base := []string{"PATH=/bin", "FOO=old", "NOEQ"}
	got := mergeEnv(base, map[string]string{"FOO": "new", "BAR": "x"})
	// PATH 保留、FOO 覆盖、NOEQ 原样、BAR 追加
	want := map[string]bool{"PATH=/bin": true, "FOO=new": true, "NOEQ": true, "BAR=x": true}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for _, kv := range got {
		if !want[kv] {
			t.Errorf("unexpected entry %q in %v", kv, got)
		}
	}
}

func TestMergeEnvEmptyExtraReturnsBase(t *testing.T) {
	base := []string{"A=1"}
	if got := mergeEnv(base, nil); !reflect.DeepEqual(got, base) {
		t.Fatalf("got %v want %v", got, base)
	}
}

func TestParseVersionParts(t *testing.T) {
	if got := parseVersionParts("1.0.0"); !reflect.DeepEqual(got, []int64{1, 0, 0}) {
		t.Errorf("1.0.0 -> %v", got)
	}
	// 非整数段跳过（对齐 Rust filter_map）
	if got := parseVersionParts("1.x.2"); !reflect.DeepEqual(got, []int64{1, 2}) {
		t.Errorf("1.x.2 -> %v", got)
	}
	if got := parseVersionParts(""); len(got) != 0 {
		t.Errorf("empty -> %v", got)
	}
}

func TestOptI32Str(t *testing.T) {
	if got := optI32Str(nil); got != "" {
		t.Errorf("nil -> %q want empty", got)
	}
	v := int32(82)
	if got := optI32Str(&v); got != "82" {
		t.Errorf("82 -> %q", got)
	}
}

func TestStrPtrDeref(t *testing.T) {
	if strPtr("") != nil {
		t.Error("empty string should map to nil pointer")
	}
	if p := strPtr("x"); p == nil || *p != "x" {
		t.Error("non-empty should map to pointer")
	}
	if derefStr(nil) != "" {
		t.Error("nil deref should be empty")
	}
}

func TestExitCode(t *testing.T) {
	if exitCode(nil) != 0 {
		t.Error("nil err -> 0")
	}
}
