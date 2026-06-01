package mcpe

import (
	"reflect"
	"strconv"
	"testing"
)

func TestBufferEvictsOldestWhenFull(t *testing.T) {
	b := newLogBuffer(3)
	b.push("a")
	b.push("b")
	b.push("c")
	b.push("d")
	if b.length() != 3 {
		t.Fatalf("length=%d want 3", b.length())
	}
	if got := b.tail(10); !reflect.DeepEqual(got, []string{"b", "c", "d"}) {
		t.Fatalf("tail=%v want [b c d]", got)
	}
}

func TestTailTakesLastN(t *testing.T) {
	b := newLogBuffer(10)
	for i := 0; i < 5; i++ {
		b.push(strconv.Itoa(i))
	}
	if got := b.tail(2); !reflect.DeepEqual(got, []string{"3", "4"}) {
		t.Fatalf("tail(2)=%v want [3 4]", got)
	}
	if got := b.tail(100); !reflect.DeepEqual(got, []string{"0", "1", "2", "3", "4"}) {
		t.Fatalf("tail(100)=%v", got)
	}
	if got := b.tail(0); len(got) != 0 {
		t.Fatalf("tail(0)=%v want empty", got)
	}
}

func TestFormatLogLineFull(t *testing.T) {
	got, ok := formatLogLine("hello", "2026-05-16 16:09:57,624", "Developer", "INFO")
	if !ok || got != "16:09:57 [Developer] [INFO] hello" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}
}

func TestFormatLogLineMinimal(t *testing.T) {
	got, ok := formatLogLine("plain", "", "", "")
	if !ok || got != "plain" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}
}

func TestFormatLogLineEmptyFiltered(t *testing.T) {
	if _, ok := formatLogLine("", "", "", ""); ok {
		t.Fatal("empty line should be filtered (ok=false)")
	}
}
