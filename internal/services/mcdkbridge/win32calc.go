package mcdkbridge

// win32calc.go — capture/click 的纯坐标/缩放计算（跨平台，便于单测）。
// Win32 调用本身在 *_windows.go；这里只放与平台无关的算术，1:1 复刻 cc-pane click.rs / capture.rs。

// clientToCoord 把客户区归一化坐标 [0,1] 映射到像素：(n*dim + 0.5) 截断取整。
// 忠实 cc-pane click.rs:149 `(nx * cw + 0.5) as i32`（Rust `as i32` 向零截断，Go int32() 同义）。
func clientToCoord(n float32, dim int32) int32 {
	return int32(n*float32(dim) + 0.5)
}

// normalizeVirtualAxis 把屏幕物理像素坐标在单轴上归一化到 0..65535（SendInput VIRTUALDESK）。
// 含 span/2 四舍五入，忠实 cc-pane click.rs:185 `((pt - origin)*65535 + span/2) / span`。
// span<=0 由调用方先行拒绝（这里假定 span>0）。
func normalizeVirtualAxis(coord, origin, span int32) int32 {
	return int32((int64(coord-origin)*65535 + int64(span)/2) / int64(span))
}

// scaledDimensions 计算「最长边压到 maxSide」后的目标尺寸（保持宽高比，至少 1px）。
// 最长边 <= maxSide 时返回原尺寸 + false（无需缩放）。忠实 cc-pane capture.rs:236-244。
func scaledDimensions(width, height, maxSide int) (int, int, bool) {
	longest := width
	if height > longest {
		longest = height
	}
	if longest <= maxSide {
		return width, height, false
	}
	scale := float64(maxSide) / float64(longest)
	nw := int(float64(width)*scale + 0.5)
	nh := int(float64(height)*scale + 0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	return nw, nh, true
}

// inUnitRange 判定归一化坐标是否落在 [0,1]（click 入参校验）。
func inUnitRange(v float32) bool { return v >= 0.0 && v <= 1.0 }
