//go:build !production

package appinfo

// devSuffix 在 dev 构建（无 production tag，含 `wails3 dev` / 裸 `go build`）下为 "-Dev"，
// 使数据目录变成 "XK-Pane-Dev"，与 release 完全隔离。
const devSuffix = "-Dev"

// IsDev 标识当前是否为开发构建。
const IsDev = true
