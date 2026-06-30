package botbridge

import (
	"reflect"
	"testing"
)

func TestBuildAnswerKeysSingleSelect(t *testing.T) {
	keys := BuildAnswerKeys([]AnswerQuestion{
		{Options: []AnswerOption{{}, {}, {}}},
	}, map[int][]int{0: {2}})

	want := []string{keyDown, keyDown, keyEnter}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %#v, want %#v", keys, want)
	}
}

func TestBuildAnswerKeysMultiSelect(t *testing.T) {
	keys := BuildAnswerKeys([]AnswerQuestion{
		{MultiSelect: true, Options: []AnswerOption{{}, {}, {}, {}}},
	}, map[int][]int{0: {3, 1, 3, -1, 99}})

	want := []string{keyDown, keySpace, keyDown, keyDown, keySpace, keyRight, keyEnter}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %#v, want %#v", keys, want)
	}
}

func TestBuildAnswerKeysMultipleQuestions(t *testing.T) {
	keys := BuildAnswerKeys([]AnswerQuestion{
		{Options: []AnswerOption{{}, {}}},
		{MultiSelect: true, Options: []AnswerOption{{}, {}, {}}},
		{Options: []AnswerOption{{}, {}, {}}},
	}, map[int][]int{
		0: {1},
		1: {0, 2},
		2: {2},
	})

	want := []string{
		keyDown, keyEnter,
		keySpace, keyDown, keyDown, keySpace, keyRight,
		keyDown, keyDown, keyEnter,
		keyEnter,
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %#v, want %#v", keys, want)
	}
}
