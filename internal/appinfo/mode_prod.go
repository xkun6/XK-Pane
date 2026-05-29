//go:build production

package appinfo

// devSuffix 在 release 构建（`-tags production`，即 `task build` / `task package`）下为空，
// 数据目录为干净的 "XK-Pane"。
const devSuffix = ""

// IsDev 标识当前是否为开发构建。
const IsDev = false
