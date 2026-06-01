package mcpe

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func keysOf(m nbtCompound) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// 从内嵌默认 level.dat 读 → 写回 → 再读，version 与字段集合保持不变。
func TestRoundTripDefaultLevelDat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "level.dat")
	if err := writeDefaultLevelDat(p); err != nil {
		t.Fatalf("write default: %v", err)
	}
	info1, err := readLevelDat(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(info1.root) == 0 {
		t.Fatal("默认 level.dat root 不应为空")
	}
	if err := writeLevelDat(p, info1); err != nil {
		t.Fatalf("write back: %v", err)
	}
	info2, err := readLevelDat(p)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if info2.version != info1.version {
		t.Errorf("version changed: %d -> %d", info1.version, info2.version)
	}
	k1, k2 := keysOf(info1.root), keysOf(info2.root)
	if len(k1) != len(k2) {
		t.Fatalf("key count changed: %d -> %d", len(k1), len(k2))
	}
	for i := range k1 {
		if k1[i] != k2[i] {
			t.Fatalf("keys differ: %v vs %v", k1, k2)
		}
	}
}

func TestMutateFields(t *testing.T) {
	p := filepath.Join(t.TempDir(), "level.dat")
	if err := writeDefaultLevelDat(p); err != nil {
		t.Fatal(err)
	}
	info, err := readLevelDat(p)
	if err != nil {
		t.Fatal(err)
	}
	info.putString("LevelName", "TEST_WORLD")
	info.putInt("GameType", 1) // creative
	info.putLong("RandomSeed", 1234567890)
	info.putBool("cheatsEnabled", true)

	if err := writeLevelDat(p, info); err != nil {
		t.Fatal(err)
	}
	info2, err := readLevelDat(p)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := info2.root["LevelName"].(nbtString); !ok || v != "TEST_WORLD" {
		t.Errorf("LevelName=%v", info2.root["LevelName"])
	}
	if v, ok := info2.root["GameType"].(nbtInt); !ok || v != 1 {
		t.Errorf("GameType=%v", info2.root["GameType"])
	}
	if v, ok := info2.root["RandomSeed"].(nbtLong); !ok || v != 1234567890 {
		t.Errorf("RandomSeed=%v", info2.root["RandomSeed"])
	}
	if v, ok := info2.root["cheatsEnabled"].(nbtByte); !ok || v != 1 {
		t.Errorf("cheatsEnabled=%v", info2.root["cheatsEnabled"])
	}
}

func TestReadTooShortReturnsError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.dat")
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil { // < 8 字节
		t.Fatal(err)
	}
	_, err := readLevelDat(p)
	if err == nil || !strings.Contains(err.Error(), "过短") {
		t.Fatalf("expected too-short error, got %v", err)
	}
}

func TestReadNonCompoundRootReturnsError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.dat")
	data := make([]byte, 8)   // 8 字节 header
	data = append(data, 0x99) // 非法 root tag
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readLevelDat(p)
	if err == nil || (!strings.Contains(err.Error(), "Compound") && !strings.Contains(err.Error(), "tag")) {
		t.Fatalf("expected non-compound error, got %v", err)
	}
}
