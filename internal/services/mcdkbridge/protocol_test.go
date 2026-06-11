package mcdkbridge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func packetEqual(a, b Packet) bool {
	return a.Type == b.Type && bytes.Equal(a.Payload, b.Payload)
}

func codeOf(err error) string {
	var ce *CodedError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ""
}

func assertRoundTrip(t *testing.T, p Packet) {
	t.Helper()
	b := p.Encode()
	decoded, consumed, err := Decode(b)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !packetEqual(decoded, p) {
		t.Fatalf("round trip mismatch: got %+v want %+v", decoded, p)
	}
	if consumed != len(b) {
		t.Fatalf("consumed=%d want %d", consumed, len(b))
	}
}

func TestRoundTripVariants(t *testing.T) {
	assertRoundTrip(t, ExecuteServerCode("print('hi')"))
	assertRoundTrip(t, ExecuteClientCode("print('你好，世界')"))
	assertRoundTrip(t, ReloadMod())
	assertRoundTrip(t, ReloadGame())
	assertRoundTrip(t, ReloadAllShaders())
	assertRoundTrip(t, ReloadSingleShader("着色器_test.fsh"))
	assertRoundTrip(t, ReloadAddonAndGame())
	assertRoundTrip(t, NewPacket(PacketType(42), []byte("payload")))
}

func TestEncodeLayoutIsBigEndian(t *testing.T) {
	p := NewPacket(PacketExecuteServerCode, []byte("abc"))
	got := p.Encode()
	// type_id = 3 (BE), length = 3 (BE)
	want := []byte{0x00, 0x03, 0x00, 0x00, 0x00, 0x03, 'a', 'b', 'c'}
	if !bytes.Equal(got, want) {
		t.Fatalf("encode layout: got %v want %v", got, want)
	}
}

func TestDecodeReturnsConsumedBytesForPipelining(t *testing.T) {
	p1 := ReloadGame()
	p2 := ExecuteClientCode("x=1")
	buf := append(p1.Encode(), p2.Encode()...)

	a, n1, err := Decode(buf)
	if err != nil {
		t.Fatalf("decode p1: %v", err)
	}
	if !packetEqual(a, p1) {
		t.Fatalf("p1 mismatch: %+v", a)
	}
	b, n2, err := Decode(buf[n1:])
	if err != nil {
		t.Fatalf("decode p2: %v", err)
	}
	if !packetEqual(b, p2) {
		t.Fatalf("p2 mismatch: %+v", b)
	}
	if n1+n2 != len(buf) {
		t.Fatalf("consumed %d+%d != %d", n1, n2, len(buf))
	}
}

func TestDecodeShortHeaderErrors(t *testing.T) {
	_, _, err := Decode([]byte{0x00, 0x03, 0x00})
	if got := codeOf(err); got != CodeProtocolIncompleteHeader {
		t.Fatalf("code=%q want %q", got, CodeProtocolIncompleteHeader)
	}
}

func TestDecodeTruncatedPayloadErrors(t *testing.T) {
	// declares length=10 but only provides 2 payload bytes
	buf := []byte{0x00, 0x03, 0x00, 0x00, 0x00, 0x0A, 'a', 'b'}
	_, _, err := Decode(buf)
	if got := codeOf(err); got != CodeProtocolIncompletePayload {
		t.Fatalf("code=%q want %q", got, CodeProtocolIncompletePayload)
	}
}

func TestUnknownTypeIDRoundTrips(t *testing.T) {
	p := NewPacket(PacketType(999), []byte("payload"))
	b := p.Encode()
	decoded, _, err := Decode(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Type != PacketType(999) {
		t.Fatalf("type=%d want 999", decoded.Type)
	}
}

// readFrom 测试 helper：从 io.Reader 读取一个完整帧（先读 6 字节头，再按 length 读 payload）。
// 生产代码是 fire-and-forget 单向发帧不解析入站，此读半边仅用于守住编解码往返对称性。
func readFrom(r io.Reader) (Packet, error) {
	var header [HeaderLen]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Packet{}, err
	}
	typeID := binary.BigEndian.Uint16(header[0:2])
	length := int(binary.BigEndian.Uint32(header[2:6]))
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return Packet{}, err
		}
	}
	return Packet{Type: PacketType(typeID), Payload: payload}, nil
}

func TestWriteReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	p := ExecuteServerCode("print('async')")
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readFrom(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !packetEqual(got, p) {
		t.Fatalf("round trip: got %+v want %+v", got, p)
	}
}

func TestWriteReadEmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	if _, err := ReloadGame().WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readFrom(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !packetEqual(got, ReloadGame()) {
		t.Fatalf("empty payload round trip: %+v", got)
	}
}
