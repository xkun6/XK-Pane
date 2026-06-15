package mcpe

// service.go — MC PE 总编排 service。忠实移植 service.rs（含 ConfigRunTask.run 12 步）。
//
// 暴露给前端的 Wails 方法 + run 状态管理 + 日志环形缓冲 + pid 跟踪都在这里。
// 持久化经 storage.Holder（不 capture *DB，holder.Current() 永远取活跃句柄）。

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/services/mcdkbridge"
	"xkpane/internal/storage"
	"xkpane/internal/utils/logx"

	"github.com/google/uuid"
)

// mcLogBufferCap 每个 run 保留的最近日志行数（启动头 30s 约 1500~2500 行，cap=2000 覆盖一次启动 + 一段稳定运行）。
const mcLogBufferCap = 2000

type runtimeRun struct {
	session *runSession
	logs    *logBuffer
}

// Service 是 MC PE 总编排服务（注册为 Wails service，前端可调）。
type Service struct {
	holder *storage.Holder
	bridge *mcdkbridge.Service // 可 nil（桥未启动时）

	emitMu sync.RWMutex
	emit   emitFunc

	runsMu sync.RWMutex
	runs   map[string]*runtimeRun

	log *slog.Logger
}

// New 构造服务（emitter / bridge 在 app 装配阶段回填）。
func New(holder *storage.Holder) *Service {
	return &Service{holder: holder, runs: map[string]*runtimeRun{}, log: logx.For("mcpe")}
}

// SetEmitter 注入事件发射器（app.Event.Emit / mirrorEmit）。
func (s *Service) SetEmitter(fn func(name string, payload any)) {
	s.emitMu.Lock()
	s.emit = fn
	s.emitMu.Unlock()
}

// SetBridge 注入 MCDK 调试桥（用于取端口注入 env + 查状态）。
func (s *Service) SetBridge(b *mcdkbridge.Service) { s.bridge = b }

func (s *Service) emitEvent(name string, payload any) {
	s.emitMu.RLock()
	fn := s.emit
	s.emitMu.RUnlock()
	if fn != nil {
		fn(name, payload)
	}
}

func (s *Service) getPkg(id string) (storage.MCPackage, bool) {
	var pkg storage.MCPackage
	if err := s.holder.Current().GORM.First(&pkg, "id = ?", id).Error; err != nil {
		return storage.MCPackage{}, false
	}
	return pkg, true
}

// ─────────────────────── 供 orchestrator 用 ───────────────────────

// ActiveRunPIDs 当前所有活跃 run 的 PID（nil pid 跳过）。orchestrator MCDK 工具用它定位 MC PE 窗口。
func (s *Service) ActiveRunPIDs() []int {
	s.runsMu.RLock()
	defer s.runsMu.RUnlock()
	var pids []int
	for _, rt := range s.runs {
		if rt.session.pid != nil {
			pids = append(pids, *rt.session.pid)
		}
	}
	return pids
}

// RecentLogs 读最近 N 行日志（已格式化）。runID="" 时取当前活跃 run（MC PE 单实例）。供 get_mc_pe_logs。
func (s *Service) RecentLogs(runID string, lines int) []string {
	s.runsMu.RLock()
	defer s.runsMu.RUnlock()
	var rt *runtimeRun
	if runID != "" {
		rt = s.runs[runID]
	} else {
		// 优先选进程仍存活的 run：避免退出宽限窗内选到正在退出的 run、把旧日志当现役返回。
		// 死 run 会在 mc_pe:exited 时被 removeRun 驱逐，正常不会与活 run 共存（兜底取任一）。
		for _, r := range s.runs {
			if rt == nil {
				rt = r
			}
			if r.session != nil && r.session.alive() {
				rt = r
				break
			}
		}
	}
	if rt == nil {
		return nil
	}
	return rt.logs.tail(lines)
}

// removeRun 把一个 run 从注册表移除。进程自然退出（MC PE 自身重启 / 崩溃 / 外部关闭——
// 只走 mc_pe:exited 不经 Stop）时由 capturingEmit 调用：否则死 run 的冻结日志缓冲长留，
// get_mc_pe_logs 会把旧日志当现役返回（「卡在旧日志」），ActiveRunPIDs 也会把死进程 PID
// 误算进活跃、害 capture/click 定位到死窗口。
func (s *Service) removeRun(runID string) {
	s.runsMu.Lock()
	delete(s.runs, runID)
	s.runsMu.Unlock()
}

// ─────────────────────── CRUD（Wails 方法） ───────────────────────

// ListPackages 列出所有 MC 包。
func (s *Service) ListPackages() ([]storage.MCPackage, error) {
	var pkgs []storage.MCPackage
	if err := s.holder.Current().GORM.Order("created_at").Find(&pkgs).Error; err != nil {
		return nil, err
	}
	return pkgs, nil
}

// CreatePackage 用项目目录创建一个默认配置的新包（name 为空时取目录名）。
func (s *Service) CreatePackage(projectPath, name string) (storage.MCPackage, error) {
	if err := validateCommandLinePath(projectPath); err != nil { // 路径会被拼到 UAC bat
		return storage.MCPackage{}, err
	}
	if !isDir(projectPath) {
		return storage.MCPackage{}, fmt.Errorf("项目目录不存在：%s", projectPath)
	}
	pkg := storage.NewMCPackage(projectPath, name)
	if err := s.holder.Current().GORM.Create(&pkg).Error; err != nil {
		return storage.MCPackage{}, err
	}
	return pkg, nil
}

// UpdatePackage 更新一个已存在的包。
func (s *Service) UpdatePackage(pkg storage.MCPackage) error {
	if err := validateCommandLinePath(pkg.ProjectPath); err != nil {
		return err
	}
	if pkg.GameExecutablePath != "" {
		if err := validateCommandLinePath(pkg.GameExecutablePath); err != nil {
			return err
		}
	}
	for _, d := range pkg.IncludedModDirs {
		if err := validateCommandLinePath(d); err != nil {
			return err
		}
	}
	if err := validateRelativeName(pkg.World.FolderName); err != nil {
		return err
	}
	if pkg.World.SourceDir != "" {
		if err := validateCommandLinePath(pkg.World.SourceDir); err != nil {
			return err
		}
	}
	if _, ok := s.getPkg(pkg.ID); !ok {
		return fmt.Errorf("MCPackage 不存在")
	}
	// GORM Save 全字段更新并自管 UpdatedAt。
	return s.holder.Current().GORM.Save(&pkg).Error
}

// DeletePackage 删除一个包。
func (s *Service) DeletePackage(id string) error {
	tx := s.holder.Current().GORM.Delete(&storage.MCPackage{}, "id = ?", id)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("MCPackage 不存在")
	}
	return nil
}

// ScanLaunchers 扫盘找网易 MC 开发启动器。
func (s *Service) ScanLaunchers() []string { return scanLaunchers() }

// MoveWorldToTrash 把包对应存档移到回收站。
func (s *Service) MoveWorldToTrash(id string) error {
	pkg, ok := s.getPkg(id)
	if !ok {
		return fmt.Errorf("MCPackage 不存在")
	}
	if err := validateRelativeName(pkg.World.FolderName); err != nil {
		return err
	}
	worlds, ok := worldsDir()
	if !ok {
		return fmt.Errorf("存档根目录不存在")
	}
	worldPath := filepath.Join(worlds, pkg.World.FolderName)
	if !isDir(worldPath) {
		return fmt.Errorf("存档不存在：%s", worldPath)
	}
	return moveToTrash(worldPath)
}

// OpenWorldFolder 返回包对应存档目录路径（前端用 NativeDialogs / shell 打开）。
func (s *Service) OpenWorldFolder(id string) (string, error) {
	pkg, ok := s.getPkg(id)
	if !ok {
		return "", fmt.Errorf("MCPackage 不存在")
	}
	if err := validateRelativeName(pkg.World.FolderName); err != nil {
		return "", err
	}
	worlds, ok := worldsDir()
	if !ok {
		return "", fmt.Errorf("存档根目录不存在")
	}
	return filepath.Join(worlds, pkg.World.FolderName), nil
}

// GetMcdkBridgeStatus 返回 MCDK 调试桥当前状态（前端运行面板轮询）。
func (s *Service) GetMcdkBridgeStatus() storage.McdkBridgeStatus {
	if s.bridge == nil {
		return storage.McdkBridgeStatus{}
	}
	st := s.bridge.Status()
	out := storage.McdkBridgeStatus{Port: s.bridge.Port(), Connected: st.Connected}
	if st.Connected {
		ra := st.RemoteAddr
		ms := st.ConnectedAtMs
		out.RemoteAddr = &ra
		out.ConnectedAtMs = &ms
	}
	return out
}

// ─────────────────────── 调试桥热重载（Wails 方法） ───────────────────────

// ReloadScript 经 MCDK 桥触发游戏内脚本热重载（typeID=1 AUTO_RELOAD → 游戏内 RELOAD_MOD，
// 等价于游戏内按「重载脚本」热键，但无需窗口聚焦）。fire-and-forget。
func (s *Service) ReloadScript() error {
	return s.sendBridgeReload(mcdkbridge.ReloadMod())
}

// ReloadWorld 经 MCDK 桥触发重启本地世界（typeID=5 → 游戏内 RELOAD_WORLD，等价于游戏内
// 按「重载世界」热键，不退游戏）。fire-and-forget。
func (s *Service) ReloadWorld() error {
	return s.sendBridgeReload(mcdkbridge.ReloadGame())
}

func (s *Service) sendBridgeReload(pkt mcdkbridge.Packet) error {
	if s.bridge == nil {
		return fmt.Errorf("MCDK 调试桥不可用")
	}
	if err := s.bridge.SendPacket(pkt); err != nil {
		return fmt.Errorf("发送热重载失败（调试桥未连入？）: %w", err)
	}
	return nil
}

// ─────────────────────── 启动主流程（12 步） ───────────────────────

// Launch 复刻 ConfigRunTask.run 的 12 步主流程。include_test_mod=true 且桥可用时注入
// MCDEV_DEBUG_IPC_PORT 让游戏内桥模组反向连入。
func (s *Service) Launch(id string) (storage.MCRunSessionInfo, error) {
	pkg, ok := s.getPkg(id)
	if !ok {
		return storage.MCRunSessionInfo{}, fmt.Errorf("MCPackage 不存在")
	}

	// 1. 校验启动器 + 所有用户输入路径（防 cmd 注入，后续走 UAC bat）
	exe := pkg.GameExecutablePath
	if exe == "" {
		return storage.MCRunSessionInfo{}, fmt.Errorf("启动器路径未配置")
	}
	if err := validateCommandLinePath(exe); err != nil {
		return storage.MCRunSessionInfo{}, err
	}
	if err := validateCommandLinePath(pkg.ProjectPath); err != nil {
		return storage.MCRunSessionInfo{}, err
	}
	for _, d := range pkg.IncludedModDirs {
		if err := validateCommandLinePath(d); err != nil {
			return storage.MCRunSessionInfo{}, err
		}
	}
	if err := validateRelativeName(pkg.World.FolderName); err != nil {
		return storage.MCRunSessionInfo{}, err
	}
	if err := validateLauncher(exe); err != nil {
		return storage.MCRunSessionInfo{}, err
	}

	// 2. 清理 symlinks
	bpDir, ok := behaviorPacksDir()
	if !ok {
		return storage.MCRunSessionInfo{}, fmt.Errorf("behavior_packs 目录不存在（请先打开开发版 MC 一次）")
	}
	rpDir, ok := resourcePacksDir()
	if !ok {
		return storage.MCRunSessionInfo{}, fmt.Errorf("resource_packs 目录不存在")
	}
	if err := removeSymlinksIn(bpDir); err != nil {
		return storage.MCRunSessionInfo{}, err
	}
	if err := removeSymlinksIn(rpDir); err != nil {
		return storage.MCRunSessionInfo{}, err
	}

	// 3. 扫描项目包 + 额外包
	packMap := map[PackType][]PackInfo{}
	if !parsePack(pkg.ProjectPath, packMap) {
		return storage.MCRunSessionInfo{}, fmt.Errorf("项目目录中未找到合法的行为包或资源包（manifest.json 缺失？）")
	}
	for _, extra := range pkg.IncludedModDirs {
		if !parsePack(extra, packMap) {
			return storage.MCRunSessionInfo{}, fmt.Errorf("附加组件目录 %q 不含合法包", extra)
		}
	}

	// 4. 重建 symlinks（权限不足的收集后一次性 UAC 提权）
	var sudoPairs []symlinkPair
	for ptype, list := range packMap {
		targetDir := bpDir
		if ptype == PackResource {
			targetDir = rpDir
		}
		for _, p := range list {
			link := filepath.Join(targetDir, p.UUID)
			created, err := createSymlink(p.Path, link)
			if err != nil {
				return storage.MCRunSessionInfo{}, err
			}
			if !created {
				sudoPairs = append(sudoPairs, symlinkPair{target: p.Path, link: link})
			}
		}
	}
	if len(sudoPairs) > 0 {
		if err := sudoCreateSymlinks(sudoPairs); err != nil {
			return storage.MCRunSessionInfo{}, err
		}
	}

	// 5. 桥模组解压（受 include_test_mod 开关控制）
	if pkg.DebugOptions.IncludeTestMod {
		bridgeDst := filepath.Join(bpDir, BridgeUUID)
		if err := ExtractTo(bridgeDst); err != nil {
			return storage.MCRunSessionInfo{}, err
		}
		bridgeMap := map[PackType][]PackInfo{}
		if !parsePack(bridgeDst, bridgeMap) {
			s.log.Warn("桥模组解压后仍未识别 manifest.json，启动可能失败")
		}
		for k, v := range bridgeMap {
			packMap[k] = append(packMap[k], v...)
		}
	}

	// 6. 存档目录
	worlds, ok := worldsDir()
	if !ok {
		return storage.MCRunSessionInfo{}, fmt.Errorf("存档根目录不存在")
	}
	worldPath := filepath.Join(worlds, pkg.World.FolderName)
	if pkg.World.SourceDir != "" {
		if err := validateCommandLinePath(pkg.World.SourceDir); err != nil {
			return storage.MCRunSessionInfo{}, err
		}
		if !isDir(pkg.World.SourceDir) {
			return storage.MCRunSessionInfo{}, fmt.Errorf("存档源目录不存在：%s", pkg.World.SourceDir)
		}
		if pkg.World.ResetOnLaunch && pathExists(worldPath) {
			s.log.Info("reset_on_launch=true，清理旧副本", "path", worldPath)
			if err := removeDirSafely(worldPath); err != nil {
				return storage.MCRunSessionInfo{}, err
			}
		}
		if !isDir(worldPath) {
			s.log.Info("从源目录复制存档", "src", pkg.World.SourceDir, "dst", worldPath)
			if err := copyWorldEssentials(pkg.World.SourceDir, worldPath); err != nil {
				return storage.MCRunSessionInfo{}, err
			}
		}
	} else if !isDir(worldPath) {
		if err := os.MkdirAll(worldPath, 0o755); err != nil {
			return storage.MCRunSessionInfo{}, fmt.Errorf("创建存档目录: %w", err)
		}
	}

	// 7. level.dat（强制 OP：permissionsLevel=1，否则调试指令被当访客拒绝）
	levelDat := filepath.Join(worldPath, "level.dat")
	isNewWorld := !isFile(levelDat)
	if isNewWorld {
		if err := writeDefaultLevelDat(levelDat); err != nil {
			return storage.MCRunSessionInfo{}, err
		}
	}
	info, err := readLevelDat(levelDat)
	if err != nil {
		return storage.MCRunSessionInfo{}, err
	}
	info.putLong("LastPlayed", time.Now().Unix())
	info.putString("LevelName", pkg.Name)
	info.putLong("RandomSeed", pkg.World.Seed)
	info.putInt("GameType", pkg.World.GameMode.Code())
	// Generator 只在新建世界时按 LevelType 写；导入的现成地图保留它自己的 Generator——
	// 网易自定义维度地图通常为 Generator=2，被覆盖成默认 1 会使自定义维度退回 0-255、
	// Y<0 地形不渲染、无法放置（dim 0 不受影响）。这是 Y<0 不显示的真正根因。
	if isNewWorld {
		info.putInt("Generator", pkg.World.LevelType.Code())
	}
	info.putBool("cheatsEnabled", pkg.World.EnableCheats)
	info.putBool("keepInventory", pkg.World.KeepInventory)
	info.putBool("dodaylightcycle", pkg.World.DoDaylightCycle)
	info.putBool("doweathercycle", pkg.World.DoWeatherCycle)
	info.putInt("permissionsLevel", 1)
	info.putInt("playerPermissionsLevel", 1)
	if err := writeLevelDat(levelDat, info); err != nil {
		return storage.MCRunSessionInfo{}, err
	}

	// 8. world_*_packs.json（按 uuid 去重，否则引擎见重复 pack_id 整包不加载）
	behPacks := make([]map[string]any, 0)
	resPacks := make([]map[string]any, 0)
	seenBeh := map[string]bool{}
	seenRes := map[string]bool{}
	for ptype, list := range packMap {
		for _, p := range list {
			seen := seenBeh
			if ptype == PackResource {
				seen = seenRes
			}
			if seen[p.UUID] {
				continue
			}
			seen[p.UUID] = true
			entry := map[string]any{"pack_id": p.UUID, "version": parseVersionParts(p.Version)}
			if ptype == PackResource {
				resPacks = append(resPacks, entry)
			} else {
				behPacks = append(behPacks, entry)
			}
		}
	}
	if err := writeJSONFile(filepath.Join(worldPath, "world_behavior_packs.json"), behPacks); err != nil {
		return storage.MCRunSessionInfo{}, fmt.Errorf("写 world_behavior_packs.json: %w", err)
	}
	if err := writeJSONFile(filepath.Join(worldPath, "world_resource_packs.json"), resPacks); err != nil {
		return storage.MCRunSessionInfo{}, fmt.Errorf("写 world_resource_packs.json: %w", err)
	}

	// 9. launch_config.cppconfig
	skinPath := filepath.Join(filepath.Dir(exe), "data", "skin_packs", "vanilla", "steve.png")
	launchCfg := map[string]any{
		"world_info":  map[string]any{"level_id": pkg.World.FolderName},
		"room_info":   map[string]any{},
		"player_info": map[string]any{"urs": "", "user_id": 0, "user_name": pkg.World.UserName},
		"skin_info":   map[string]any{"slim": false, "skin": skinPath},
	}
	cppconfigPath := filepath.Join(worldPath, "launch_config.cppconfig")
	if err := writeJSONFile(cppconfigPath, launchCfg); err != nil {
		return storage.MCRunSessionInfo{}, fmt.Errorf("写 launch_config.cppconfig: %w", err)
	}

	// 10. 环境变量（reload_key/reload_world_key 为数字，reload_addon_key/reload_shaders_key 为字符串——忠实 cc-pane）
	debugOpts := map[string]any{
		"reload_key":         pkg.DebugOptions.ReloadKey,
		"reload_world_key":   pkg.DebugOptions.ReloadWorldKey,
		"reload_addon_key":   optI32Str(pkg.DebugOptions.ReloadAddonKey),
		"reload_shaders_key": optI32Str(pkg.DebugOptions.ReloadShadersKey),
		"reload_key_global":  pkg.DebugOptions.ReloadKeyGlobal,
	}
	targetModDirs := []string{strings.ReplaceAll(pkg.ProjectPath, "\\", "/")} // 反斜杠转正斜杠（忠实）
	env := map[string]string{}
	if dj, err := json.Marshal(debugOpts); err == nil {
		env["MCS_HELPER_DEBUG_OPTIONS"] = string(dj)
	}
	if tj, err := json.Marshal(targetModDirs); err == nil {
		env["MCS_HELPER_TARGET_MOD_DIRS"] = string(tj)
	}
	if pkg.DebugOptions.IncludeTestMod && s.bridge != nil {
		if port := s.bridge.Port(); port > 0 {
			env["MCDEV_DEBUG_IPC_PORT"] = strconv.Itoa(port)
		}
	}

	// 11. 启动（emitter 包装：把本 run 的 mc_pe:output 也抄进环形缓冲供 MCP/HTTP 读取）
	args := []string{"config=" + cppconfigPath}
	runID := uuid.NewString()
	buf := newLogBuffer(mcLogBufferCap)
	capturingEmit := func(name string, payload any) {
		switch name {
		case events.MCPeOutput:
			if op, ok := payload.(events.MCOutputPayload); ok && op.RunID == runID {
				if line, ok := formatLogLine(op.Line, derefStr(op.Timestamp), derefStr(op.Category), derefStr(op.Level)); ok {
					buf.push(line)
				}
			}
		case events.MCPeExited:
			// 进程退出即驱逐本 run，别让冻结的旧日志缓冲被 get_mc_pe_logs 当现役返回（见 removeRun）。
			if ep, ok := payload.(events.MCExitedPayload); ok && ep.RunID == runID {
				s.removeRun(runID)
			}
		}
		s.emitEvent(name, payload)
	}
	session, err := spawnRun(runID, pkg.ID, exe, args, env, capturingEmit)
	if err != nil {
		return storage.MCRunSessionInfo{}, err
	}

	s.runsMu.Lock()
	s.runs[runID] = &runtimeRun{session: session, logs: buf}
	s.runsMu.Unlock()

	s.emitEvent(events.MCPeLaunched, events.MCLaunchedPayload{RunID: runID, PackageID: pkg.ID})
	return storage.MCRunSessionInfo{RunID: runID, PackageID: pkg.ID, PID: session.pid}, nil
}

// Stop 终止指定 run。
func (s *Service) Stop(runID string) error {
	s.runsMu.Lock()
	rt := s.runs[runID]
	delete(s.runs, runID)
	s.runsMu.Unlock()
	if rt != nil {
		rt.session.kill()
	}
	return nil
}

// CleanupAll 应用退出时杀掉所有 MC 子进程。
func (s *Service) CleanupAll() {
	s.runsMu.Lock()
	all := s.runs
	s.runs = map[string]*runtimeRun{}
	s.runsMu.Unlock()
	for _, rt := range all {
		rt.session.kill()
	}
}

// ─────────────────────── helpers ───────────────────────

func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func optI32Str(p *int32) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(int(*p))
}

func parseVersionParts(v string) []int64 {
	var out []int64
	for _, seg := range strings.Split(v, ".") {
		if n, err := strconv.ParseInt(seg, 10, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func writeJSONFile(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
