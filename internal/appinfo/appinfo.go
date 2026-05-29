// Package appinfo 提供编译期解析的应用身份常量（单一事实源）。
//
// 通过 build tag 区分 dev / production 构建，使两者的数据目录、主密钥、日志目录
// 互不冲突，可同时运行（对标 cc-pane 的 ~/.cc-panes vs ~/.cc-panes-dev）。
//
// 信号来源：release 构建带 `-tags production`（见 build/windows/Taskfile.yml 的
// build:native —— 非 DEV 分支加 `-tags production`；`wails3 dev` 走 DEV=true 分支
// 不加该 tag）。因此 `//go:build production` = release，`//go:build !production` = dev。
//
// 本包是零依赖叶子包：storage / cryptox / logx 均可安全 import，不形成环
// （这正是过去 cryptox 自带独立常量想规避的 cryptox→storage 反向依赖；改用中立
// 叶子包后，单一事实源 + 无环两者兼得）。改业务名只需改这里的 BaseName。
package appinfo

// BaseName 应用基础名：数据目录 / 主密钥 / 日志目录的根名。
const BaseName = "XK-Pane"

// AppDirName 是写到用户数据目录下的子文件夹名。
// release = BaseName；dev = BaseName + "-Dev"（见 mode_dev.go / mode_prod.go）。
const AppDirName = BaseName + devSuffix
