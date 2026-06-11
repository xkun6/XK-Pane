[根目录](../CLAUDE.md) > **build**

# build - 多平台构建模块

## 模块职责

集中维护跨平台构建、打包、分发所需的所有脚本、配置与资源。每个平台拥有独立的 `Taskfile.yml`，由根 `Taskfile.yml` 通过 `includes` 聚合调用。打包目标涵盖 Windows (NSIS)、macOS (Info.plist / Assets.car)、Linux (AppImage / nfpm)、以及 Docker 服务器/跨编译镜像。（iOS / Android 构建链与 MSIX 打包链路已于 2026-06-11 减法审计中移除，需要时按 git 历史恢复。）

## 入口与启动

- 公共入口：`build/Taskfile.yml`（被根 Taskfile 通过 `common:` 命名空间引用）
- 全局应用元信息：`build/config.yml`（`info.companyName`、`productName`、`version` 等；修改后需要 `wails3 task common:update:build-assets` 同步资源）
- 应用图标：`build/appicon.png`、`build/appicon.icon/`

## 对外接口

通过根 Taskfile 暴露给开发者的命令均最终调用本目录下的子 Taskfile：

| 子模块 | 入口 Taskfile | 主要产物 |
|--------|---------------|----------|
| Windows | `build/windows/Taskfile.yml` | `nsis/project.nsi`、`wails.exe.manifest` |
| macOS | `build/darwin/Taskfile.yml` | `Info.plist`、`Info.dev.plist`、`Assets.car`、`icons.icns` |
| Linux | `build/linux/Taskfile.yml` | `appimage/build.sh`、`nfpm/nfpm.yaml`、`desktop/` |
| Docker | `build/docker/` | `Dockerfile.server`、`Dockerfile.cross` |

## 关键依赖与配置

- `build/config.yml`
  - `info.*`：版本、公司、Bundle ID、Copyright
  - `dev_mode`：dev server 监视/忽略规则、子进程启动顺序
  - `fileAssociations`：文件关联占位
- 安装钩子（Linux）：`build/linux/nfpm/scripts/{preinstall,postinstall,preremove,postremove}.sh`

## 数据模型

无应用层数据模型。仅包含构建系统的元数据与资源声明（plist、manifest、proto 等）。

## 测试与质量

- 无构建脚本测试。
- 建议：在 CI 中分别针对 Windows / macOS / Linux 跑 `task build`，验证打包配置可用。
- `build/windows/nsis/MicrosoftEdgeWebview2Setup.exe` 已在 `.gitignore` 中（运行时下载/缓存）。

## 常见问题 (FAQ)

- Q：修改了 `build/config.yml` 中的 `info.*` 字段，但二进制元数据没变？
  - A：执行 `wails3 task common:update:build-assets` 重新生成各平台资源（plist、manifest 等）。注意：该命令可能把已删除的 iOS/MSIX 模板资源重新生成回来，回流无害，再删一次即可。
- Q：如何切换包管理器？
  - A：根 Taskfile 的 `PACKAGE_MANAGER` 默认 `pnpm`，可在调用任务时通过 `task PACKAGE_MANAGER=npm dev` 覆盖。

## 相关文件清单

```
build/
├── Taskfile.yml                   # 公共构建任务
├── config.yml                     # 应用元信息与 dev_mode 配置
├── appicon.png / appicon.icon/    # 应用图标源
├── windows/
│   ├── Taskfile.yml
│   ├── icon.ico
│   ├── info.json
│   ├── wails.exe.manifest
│   └── nsis/{project.nsi, wails_tools.nsh}
├── darwin/
│   ├── Taskfile.yml
│   ├── Info.plist / Info.dev.plist
│   ├── Assets.car
│   └── icons.icns
├── linux/
│   ├── Taskfile.yml
│   ├── appimage/build.sh
│   ├── desktop/
│   └── nfpm/{nfpm.yaml, scripts/*.sh}
└── docker/
    ├── Dockerfile.server          # 服务器模式镜像
    └── Dockerfile.cross           # 跨编译镜像（约 800MB）
```

## 变更记录 (Changelog)

- 2026-06-11：减法审计批 3——移除 `build/ios/`、`build/android/` 整目录与根 Taskfile 对应 include、MSIX 打包链路（`build/windows/msix/` + 两个 task，原引用不存在的 wails.json 本就损坏）、死任务 `frontend:vendor:puppertino` 与 `ios:device:list` / `ios:run:device`；`windows:package` 收敛为仅 NSIS。详见 SUBTRACTION_AUDIT.md #51-#53。
- 2026-05-28 13:09:44：初始化模块文档。
