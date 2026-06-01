package mcpe

// leveldata.go — Bedrock level.dat 读写。忠实移植 level_data.rs。
//
// Bedrock level.dat 文件结构：
//   - 8 字节头：int32 LE 版本 + int32 LE NBT 字节数
//   - 之后是 Little-Endian NBT payload，根标签是带空 name 的 Compound
//
// 这里实现一个最小够用的 Bedrock LE NBT 读/写器（只支持 level.dat 实际出现的 tag 类型），
// 不支持 Java BE 模式。注意：与 mcdkbridge 的桥帧（大端）相反，NBT 全程 LittleEndian。

import (
	"bytes"
	_ "embed" // for //go:embed resources/level.dat
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

const (
	tagEnd       uint8 = 0x00
	tagByte      uint8 = 0x01
	tagShort     uint8 = 0x02
	tagInt       uint8 = 0x03
	tagLong      uint8 = 0x04
	tagFloat     uint8 = 0x05
	tagDouble    uint8 = 0x06
	tagByteArray uint8 = 0x07
	tagString    uint8 = 0x08
	tagList      uint8 = 0x09
	tagCompound  uint8 = 0x0A
	tagIntArray  uint8 = 0x0B
	tagLongArray uint8 = 0x0C
)

// nbtValue 是 NBT 值的密封集合（每个类型自报 tag id，对齐 Rust enum NbtValue）。
type nbtValue interface{ tagID() uint8 }

type (
	nbtByte      int8
	nbtShort     int16
	nbtInt       int32
	nbtLong      int64
	nbtFloat     float32
	nbtDouble    float64
	nbtByteArray []int8
	nbtString    string
	nbtList      struct {
		tagType uint8 // 用于空列表保留类型；非空时与 items 一致
		items   []nbtValue
	}
	nbtCompound  map[string]nbtValue
	nbtIntArray  []int32
	nbtLongArray []int64
)

func (nbtByte) tagID() uint8      { return tagByte }
func (nbtShort) tagID() uint8     { return tagShort }
func (nbtInt) tagID() uint8       { return tagInt }
func (nbtLong) tagID() uint8      { return tagLong }
func (nbtFloat) tagID() uint8     { return tagFloat }
func (nbtDouble) tagID() uint8    { return tagDouble }
func (nbtByteArray) tagID() uint8 { return tagByteArray }
func (nbtString) tagID() uint8    { return tagString }
func (nbtList) tagID() uint8      { return tagList }
func (nbtCompound) tagID() uint8  { return tagCompound }
func (nbtIntArray) tagID() uint8  { return tagIntArray }
func (nbtLongArray) tagID() uint8 { return tagLongArray }

// tagInfo 持有 level.dat 头 + 根 Compound 内容（不含外层 tag_id / name）。
type tagInfo struct {
	version int32
	size    int32
	root    nbtCompound
}

func (t *tagInfo) putLong(key string, v int64) { t.root[key] = nbtLong(v) }
func (t *tagInfo) putInt(key string, v int32)  { t.root[key] = nbtInt(v) }
func (t *tagInfo) putString(key, v string)     { t.root[key] = nbtString(v) }
func (t *tagInfo) putBool(key string, v bool) { // Bedrock 无原生 bool，用 Byte 0/1
	if v {
		t.root[key] = nbtByte(1)
	} else {
		t.root[key] = nbtByte(0)
	}
}

// ───────────────────────── 读 ─────────────────────────

func readNBTString(r io.Reader) (string, error) {
	var n uint16
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", fmt.Errorf("NBT read string length: %w", err)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("NBT read string body: %w", err)
	}
	if !utf8.Valid(buf) {
		return "", fmt.Errorf("NBT string not UTF-8")
	}
	return string(buf), nil
}

func readNBTPayload(r io.Reader, tag uint8) (nbtValue, error) {
	le := binary.LittleEndian
	switch tag {
	case tagByte:
		var v int8
		err := binary.Read(r, le, &v)
		return nbtByte(v), err
	case tagShort:
		var v int16
		err := binary.Read(r, le, &v)
		return nbtShort(v), err
	case tagInt:
		var v int32
		err := binary.Read(r, le, &v)
		return nbtInt(v), err
	case tagLong:
		var v int64
		err := binary.Read(r, le, &v)
		return nbtLong(v), err
	case tagFloat:
		var v float32
		err := binary.Read(r, le, &v)
		return nbtFloat(v), err
	case tagDouble:
		var v float64
		err := binary.Read(r, le, &v)
		return nbtDouble(v), err
	case tagByteArray:
		n, err := readArrayLen(r)
		if err != nil {
			return nil, err
		}
		v := make([]int8, n)
		if err := binary.Read(r, le, v); err != nil {
			return nil, err
		}
		return nbtByteArray(v), nil
	case tagString:
		s, err := readNBTString(r)
		return nbtString(s), err
	case tagList:
		var itemTag uint8
		if err := binary.Read(r, le, &itemTag); err != nil {
			return nil, err
		}
		n, err := readArrayLen(r)
		if err != nil {
			return nil, err
		}
		items := make([]nbtValue, 0, n)
		for i := 0; i < n; i++ {
			it, err := readNBTPayload(r, itemTag)
			if err != nil {
				return nil, err
			}
			items = append(items, it)
		}
		return nbtList{tagType: itemTag, items: items}, nil
	case tagCompound:
		return readNBTCompound(r)
	case tagIntArray:
		n, err := readArrayLen(r)
		if err != nil {
			return nil, err
		}
		v := make([]int32, n)
		if err := binary.Read(r, le, v); err != nil {
			return nil, err
		}
		return nbtIntArray(v), nil
	case tagLongArray:
		n, err := readArrayLen(r)
		if err != nil {
			return nil, err
		}
		v := make([]int64, n)
		if err := binary.Read(r, le, v); err != nil {
			return nil, err
		}
		return nbtLongArray(v), nil
	default:
		return nil, fmt.Errorf("未知 NBT tag id: 0x%02X", tag)
	}
}

// readArrayLen 读 int32 LE 长度，负数按 0 处理（防御损坏文件，对齐 Rust .max(0)）。
func readArrayLen(r io.Reader) (int, error) {
	var n int32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, nil
	}
	return int(n), nil
}

func readNBTCompound(r io.Reader) (nbtCompound, error) {
	out := nbtCompound{}
	for {
		var tag uint8
		if err := binary.Read(r, binary.LittleEndian, &tag); err != nil {
			return nil, err
		}
		if tag == tagEnd {
			return out, nil
		}
		name, err := readNBTString(r)
		if err != nil {
			return nil, err
		}
		value, err := readNBTPayload(r, tag)
		if err != nil {
			return nil, err
		}
		out[name] = value
	}
}

// ───────────────────────── 写 ─────────────────────────

func writeNBTString(w io.Writer, s string) error {
	b := []byte(s)
	if err := binary.Write(w, binary.LittleEndian, uint16(len(b))); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

func writeNBTPayload(w io.Writer, v nbtValue) error {
	le := binary.LittleEndian
	switch x := v.(type) {
	case nbtByte:
		return binary.Write(w, le, int8(x))
	case nbtShort:
		return binary.Write(w, le, int16(x))
	case nbtInt:
		return binary.Write(w, le, int32(x))
	case nbtLong:
		return binary.Write(w, le, int64(x))
	case nbtFloat:
		return binary.Write(w, le, float32(x))
	case nbtDouble:
		return binary.Write(w, le, float64(x))
	case nbtByteArray:
		if err := binary.Write(w, le, int32(len(x))); err != nil {
			return err
		}
		return binary.Write(w, le, []int8(x))
	case nbtString:
		return writeNBTString(w, string(x))
	case nbtList:
		if err := binary.Write(w, le, x.tagType); err != nil {
			return err
		}
		if err := binary.Write(w, le, int32(len(x.items))); err != nil {
			return err
		}
		for _, it := range x.items {
			if err := writeNBTPayload(w, it); err != nil {
				return err
			}
		}
		return nil
	case nbtCompound:
		return writeNBTCompound(w, x)
	case nbtIntArray:
		if err := binary.Write(w, le, int32(len(x))); err != nil {
			return err
		}
		return binary.Write(w, le, []int32(x))
	case nbtLongArray:
		if err := binary.Write(w, le, int32(len(x))); err != nil {
			return err
		}
		return binary.Write(w, le, []int64(x))
	default:
		return fmt.Errorf("不支持的 NBT 值类型 %T", v)
	}
}

func writeNBTCompound(w io.Writer, m nbtCompound) error {
	// HashMap/map 无序，Bedrock 写入顺序不影响读取（按 name 索引）。
	for name, value := range m {
		if err := binary.Write(w, binary.LittleEndian, value.tagID()); err != nil {
			return err
		}
		if err := writeNBTString(w, name); err != nil {
			return err
		}
		if err := writeNBTPayload(w, value); err != nil {
			return err
		}
	}
	return binary.Write(w, binary.LittleEndian, tagEnd)
}

// ───────────────────────── 公共 API ─────────────────────────

// 内嵌默认 level.dat（写新世界时使用）。
//
//go:embed resources/level.dat
var defaultLevelDat []byte

// readLevelDat 读取 level.dat（含 8 字节 Bedrock header）。
func readLevelDat(path string) (*tagInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 level.dat: %w", err)
	}
	if len(data) < 8 {
		return nil, fmt.Errorf("level.dat 文件过短")
	}
	cur := bytes.NewReader(data)
	var version, size int32
	if err := binary.Read(cur, binary.LittleEndian, &version); err != nil {
		return nil, err
	}
	if err := binary.Read(cur, binary.LittleEndian, &size); err != nil {
		return nil, err
	}
	var rootTag uint8
	if err := binary.Read(cur, binary.LittleEndian, &rootTag); err != nil {
		return nil, err
	}
	if rootTag != tagCompound {
		return nil, fmt.Errorf("level.dat root 不是 Compound（tag 0x%02X）", rootTag)
	}
	if _, err := readNBTString(cur); err != nil { // 根 name，通常为空
		return nil, err
	}
	root, err := readNBTCompound(cur)
	if err != nil {
		return nil, err
	}
	return &tagInfo{version: version, size: size, root: root}, nil
}

// writeLevelDat 写入 level.dat（重新计算 size header）。
func writeLevelDat(path string, info *tagInfo) error {
	var payload bytes.Buffer
	payload.WriteByte(tagCompound)
	if err := writeNBTString(&payload, ""); err != nil { // root name = ""
		return err
	}
	if err := writeNBTCompound(&payload, info.root); err != nil {
		return err
	}

	var out bytes.Buffer
	if err := binary.Write(&out, binary.LittleEndian, info.version); err != nil {
		return err
	}
	if err := binary.Write(&out, binary.LittleEndian, int32(payload.Len())); err != nil {
		return err
	}
	out.Write(payload.Bytes())

	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return fmt.Errorf("写入 level.dat: %w", err)
	}
	return nil
}

// writeDefaultLevelDat 把内嵌的默认 level.dat 写到目标路径。
func writeDefaultLevelDat(target string) error {
	if parent := filepath.Dir(target); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(target, defaultLevelDat, 0o644); err != nil {
		return fmt.Errorf("写入默认 level.dat: %w", err)
	}
	return nil
}
