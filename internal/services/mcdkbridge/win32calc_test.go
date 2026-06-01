package mcdkbridge

import "testing"

func TestClientToCoord(t *testing.T) {
	cases := []struct {
		n    float32
		dim  int32
		want int32
	}{
		{0, 100, 0},
		{1, 100, 100},  // int32(100.5) 截断 → 100
		{0.5, 100, 50}, // int32(50.5) → 50
		{0.5, 200, 100},
		{0.123, 1920, 236}, // int32(0.123*1920+0.5)=int32(236.66)=236
	}
	for _, c := range cases {
		if got := clientToCoord(c.n, c.dim); got != c.want {
			t.Errorf("clientToCoord(%v,%d) = %d, want %d", c.n, c.dim, got, c.want)
		}
	}
}

func TestNormalizeVirtualAxis(t *testing.T) {
	cases := []struct {
		coord, origin, span, want int32
	}{
		{0, 0, 1920, 0},
		{1920, 0, 1920, 65535},
		{960, 0, 1920, 32768},    // 中点四舍五入
		{100, 100, 1000, 0},      // origin 偏移归零
		{1100, 100, 1000, 65535}, // (1000*65535+500)/1000 = 65535
	}
	for _, c := range cases {
		if got := normalizeVirtualAxis(c.coord, c.origin, c.span); got != c.want {
			t.Errorf("normalizeVirtualAxis(%d,%d,%d) = %d, want %d", c.coord, c.origin, c.span, got, c.want)
		}
	}
}

func TestScaledDimensions(t *testing.T) {
	cases := []struct {
		w, h, max    int
		wantW, wantH int
		wantNeed     bool
	}{
		{1920, 1080, 480, 480, 270, true},
		{960, 480, 480, 480, 240, true},
		{400, 300, 480, 400, 300, false}, // 最长边 ≤ max，不缩放
		{480, 480, 480, 480, 480, false}, // 等于 max 不缩放
		{481, 10, 480, 480, 10, true},    // 极端宽高比，至少 1px：int(10*480/481+0.5)=10
	}
	for _, c := range cases {
		gw, gh, need := scaledDimensions(c.w, c.h, c.max)
		if gw != c.wantW || gh != c.wantH || need != c.wantNeed {
			t.Errorf("scaledDimensions(%d,%d,%d) = (%d,%d,%v), want (%d,%d,%v)",
				c.w, c.h, c.max, gw, gh, need, c.wantW, c.wantH, c.wantNeed)
		}
	}
}

func TestInUnitRange(t *testing.T) {
	for _, v := range []float32{0, 1, 0.5} {
		if !inUnitRange(v) {
			t.Errorf("inUnitRange(%v) = false, want true", v)
		}
	}
	for _, v := range []float32{-0.001, 1.001, -1, 2} {
		if inUnitRange(v) {
			t.Errorf("inUnitRange(%v) = true, want false", v)
		}
	}
}
