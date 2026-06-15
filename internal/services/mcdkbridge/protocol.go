// Package mcdkbridge 是 MCDK 调试桥（对齐 cc-pane cc-panes-core/src/services/mcdk_bridge）。
//
// 它 bind 127.0.0.1:0 起一个 TCP server，等游戏内 Python 桥模组（IPCSystem.py）反向连入，
// 然后对其 fire-and-forget 发送 6 字节大端帧（执行 Python 代码 / 热重载）。截图 / 点击
// （capture/click，Windows-only）是独立旁路，不走本桥（见后续单元）。
//
// 本文件 = 帧编解码层，忠实移植 protocol.rs：
//
//	帧格式（大端 BigEndian）：
//	  +--------+--------+
//	  | TypeID |  u16   |
//	  +--------+--------+
//	  | Length |  u32   |
//	  +--------+--------+
//	  | Payload (Length 字节) |
//	  +------------------------+
//
// payload 为裸 UTF-8 字节（非 JSON、无长度前缀、无 NUL 尾）。与游戏内 IPCSystem.py 的
// U16_BE / U32_BE 拆包完全对称。
package mcdkbridge

import (
	"encoding/binary"
	"fmt"
	"io"
)

// HeaderLen 帧头长度（u16 type_id + u32 length）。
const HeaderLen = 6

// PacketType 数据帧类型。type id 1、3..8（0/2 未使用，2 为保留空位）。1=AUTO_RELOAD
// 触发游戏内 RELOAD_MOD 脚本热重载——cc-pane 原版 C 端不发此型（仅游戏内保存时自触发），
// XK 额外用它支撑运行面板「热重载脚本」按钮。Go 用 uint16 承载，未知值原样透传（对齐
// cc-pane PacketType::Unknown 的无损往返语义）。
type PacketType uint16

const (
	PacketReloadMod          PacketType = 1
	PacketExecuteServerCode  PacketType = 3
	PacketExecuteClientCode  PacketType = 4
	PacketReloadGame         PacketType = 5
	PacketReloadAllShaders   PacketType = 6
	PacketReloadSingleShader PacketType = 7
	PacketReloadAddonAndGame PacketType = 8

	// PacketLogLine 是反向帧（游戏内桥模组 → XK-Pane）：桥模组把每行 client stdout 经
	// STD_OUT_WRAPPER tee 过来，payload=UTF-8 单行。Route A 最小验证：证明 MC Studio
	// 劫持 stdout 时，mod 日志仍能在进程内截获并经本桥送回。原桥是单向（XK→游戏），
	// 这是第一个游戏→XK 方向的帧类型。
	PacketLogLine PacketType = 100
)

// Packet IPC 数据帧。
type Packet struct {
	Type    PacketType
	Payload []byte
}

// NewPacket 构造一个帧。
func NewPacket(t PacketType, payload []byte) Packet { return Packet{Type: t, Payload: payload} }

// 语义构造器 —— 代码型 payload 为 UTF-8 原始字节（Go string 本就 UTF-8）。

func ExecuteServerCode(code string) Packet { return NewPacket(PacketExecuteServerCode, []byte(code)) }
func ExecuteClientCode(code string) Packet { return NewPacket(PacketExecuteClientCode, []byte(code)) }
func ReloadMod() Packet                    { return NewPacket(PacketReloadMod, nil) }
func ReloadGame() Packet                   { return NewPacket(PacketReloadGame, nil) }
func ReloadAllShaders() Packet             { return NewPacket(PacketReloadAllShaders, nil) }
func ReloadSingleShader(fileName string) Packet {
	return NewPacket(PacketReloadSingleShader, []byte(fileName))
}
func ReloadAddonAndGame() Packet { return NewPacket(PacketReloadAddonAndGame, nil) }

// Encode 编码为字节流（同步）。
func (p Packet) Encode() []byte {
	out := make([]byte, HeaderLen+len(p.Payload))
	binary.BigEndian.PutUint16(out[0:2], uint16(p.Type))
	binary.BigEndian.PutUint32(out[2:6], uint32(len(p.Payload)))
	copy(out[HeaderLen:], p.Payload)
	return out
}

// Decode 从字节流解码一个帧，返回 (Packet, 消耗字节数)。
//
// buffer 不完整（不足 6 字节头或 payload 截断）时返回 *CodedError，调用方可据 code 判断
// 是否需要继续累积数据（支持流水线分帧）。
func Decode(b []byte) (Packet, int, error) {
	if len(b) < HeaderLen {
		return Packet{}, 0, coded(CodeProtocolIncompleteHeader,
			fmt.Sprintf("frame header requires %d bytes, got %d", HeaderLen, len(b)))
	}
	typeID := binary.BigEndian.Uint16(b[0:2])
	length := int(binary.BigEndian.Uint32(b[2:6]))
	total := HeaderLen + length
	if len(b) < total {
		return Packet{}, 0, coded(CodeProtocolIncompletePayload,
			fmt.Sprintf("frame payload requires %d bytes, only %d available", length, len(b)-HeaderLen))
	}
	payload := make([]byte, length)
	copy(payload, b[HeaderLen:total])
	return Packet{Type: PacketType(typeID), Payload: payload}, total, nil
}

// WriteTo 把整帧写入任意 io.Writer（实现 io.WriterTo）。io.Writer.Write 契约保证
// 「写入不足即返回错误」，故单次 Write 即等价于 cc-pane 的 write_all。
//
// 注：Go 侧桥是 fire-and-forget 单向发帧，生产代码不解析入站帧；入站读半边
// （protocol.rs 的 read 对应物）只保留在 protocol_test.go 的 readFrom 测试 helper 里，
// 用于守住「线上格式与游戏内 IPCSystem.py 拆包对称」的往返覆盖。
func (p Packet) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(p.Encode())
	return int64(n), err
}
