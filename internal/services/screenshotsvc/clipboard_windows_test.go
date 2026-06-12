//go:build windows

package screenshotsvc

// buildDIBPayload 纯函数测试：BITMAPINFOHEADER 字段（小端）+ 自底向上 BGRA 行序 +
// SubImage（非零 Rect.Min、外部 Stride）取像素正确。

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestBuildDIBPayloadHeaderAndPixels(t *testing.T) {
	// 1 列 × 2 行：上红 (255,0,0)、下蓝 (0,0,255) —— 自底向上后蓝行在前。
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	img.SetRGBA(0, 1, color.RGBA{B: 255, A: 255})

	buf := buildDIBPayload(img)
	if len(buf) != 40+1*4*2 {
		t.Fatalf("负载长度 = %d，期望 48", len(buf))
	}
	if got := binary.LittleEndian.Uint32(buf[0:]); got != 40 {
		t.Errorf("biSize = %d，期望 40", got)
	}
	if got := int32(binary.LittleEndian.Uint32(buf[4:])); got != 1 {
		t.Errorf("biWidth = %d，期望 1", got)
	}
	if got := int32(binary.LittleEndian.Uint32(buf[8:])); got != 2 {
		t.Errorf("biHeight = %d，期望 2（正高 = 自底向上）", got)
	}
	if got := binary.LittleEndian.Uint16(buf[12:]); got != 1 {
		t.Errorf("biPlanes = %d，期望 1", got)
	}
	if got := binary.LittleEndian.Uint16(buf[14:]); got != 32 {
		t.Errorf("biBitCount = %d，期望 32", got)
	}
	if got := binary.LittleEndian.Uint32(buf[16:]); got != biRGB {
		t.Errorf("biCompression = %d，期望 BI_RGB", got)
	}
	if got := binary.LittleEndian.Uint32(buf[20:]); got != 8 {
		t.Errorf("biSizeImage = %d，期望 8", got)
	}

	// 像素区：第一行 = 图像底行（蓝 → BGRA: FF 00 00 FF），第二行 = 顶行（红 → 00 00 FF FF）。
	want := []byte{0xFF, 0x00, 0x00, 0xFF, 0x00, 0x00, 0xFF, 0xFF}
	for i, b := range want {
		if buf[40+i] != b {
			t.Fatalf("像素字节[%d] = %#02x，期望 %#02x（自底向上 BGRA）", i, buf[40+i], b)
		}
	}
}

func TestBuildDIBPayloadSubImage(t *testing.T) {
	// 4×4 底图，(1,1)-(3,3) 子图：验证非零 Rect.Min + 原 Stride 下取的是子图像素。
	base := image.NewRGBA(image.Rect(0, 0, 4, 4))
	base.SetRGBA(1, 1, color.RGBA{R: 10, G: 20, B: 30, A: 255}) // 子图左上
	base.SetRGBA(2, 2, color.RGBA{R: 40, G: 50, B: 60, A: 255}) // 子图右下
	sub := base.SubImage(image.Rect(1, 1, 3, 3)).(*image.RGBA)

	buf := buildDIBPayload(sub)
	if len(buf) != 40+2*4*2 {
		t.Fatalf("负载长度 = %d，期望 56", len(buf))
	}
	if got := int32(binary.LittleEndian.Uint32(buf[4:])); got != 2 {
		t.Errorf("biWidth = %d，期望 2", got)
	}
	// 自底向上：行0 = 子图第 1 行（含右下像素在 x=1），行1 = 子图第 0 行（含左上像素在 x=0）。
	px := func(row, col int) []byte {
		off := 40 + row*8 + col*4
		return buf[off : off+4]
	}
	if got := px(0, 1); got[0] != 60 || got[1] != 50 || got[2] != 40 {
		t.Errorf("底行右像素 BGR = %v，期望 [60 50 40 255]", got)
	}
	if got := px(1, 0); got[0] != 30 || got[1] != 20 || got[2] != 10 {
		t.Errorf("顶行左像素 BGR = %v，期望 [30 20 10 255]", got)
	}
}
