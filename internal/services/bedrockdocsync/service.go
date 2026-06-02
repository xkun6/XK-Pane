package bedrockdocsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/utils/filex"
)

// service.go：OTA 同步编排器（对齐 service.rs BedrockDocSyncService）。把 gitea/http 引擎、
// 索引构建器（build_api_index / build_templates_index）、manifest 串成一次 OTA：
//
//	discover 源 → gitea/http 抓到 staging → 条件构建 *-index.json → validating →
//	原子换目录（tmp→bak→live→删 bak）→ SetDataRoot 热切 bedrock 查询层 → 写 manifest → completed。
//
// 默认源列表 = [mcguide, mconline]（以代码为准，非模块注释里的三源）。无 git2：mcguide/mconline 走
// Gitea API（gitea.go），mcdocs 走 VuePress http（http_sync.go）。bedrock 注入用窄接口 BedrockHooks
// 回调（SetDataRoot + RuntimeStats），本文件**不 import bedrock**以免循环依赖。
//
// 取消：cc-pane 用 Arc<AtomicBool> 在阶段边界轮询；XK 改 context.Context —— Cancel() 取消当前 run 的
// ctx，引擎在派发循环里也响应 ctx（取消更灵敏），但对外契约（取消→Cancelled 事件 + 清 staging + 返错）
// 与 cc-pane 一致。

// BedrockHooks 是注入 bedrock 查询层的窄接口（避免 bedrockdocsync → bedrock 循环 import）。
// SetDataRoot("" 表示清空回退内嵌基线)；RuntimeStats 返回当前可服务计数（app 装配处适配 bedrock.Stats）。
type BedrockHooks struct {
	SetDataRoot  func(root string)
	RuntimeStats func() ManifestStats
}

// SkippedSource 是一条「未同步源 + 原因」（返前端 read-model，camelCase）。
type SkippedSource struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

// FetchFailureDetail 是一条逐文件抓取失败明细（带 source 标签，供面板精确定位是哪个文件失败）。
// 比 PartialErrors 的「N files failed」汇总更细：source = 引擎标签（wiki[...] / mcdocs），path =
// 失败的仓库路径 / 页面 URL，error = 原始错误。
type FetchFailureDetail struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	Error  string `json:"error"`
}

// SyncReport 是 Sync 的同步结果（对齐 SyncReport；返前端 read-model，camelCase）。
type SyncReport struct {
	ManifestPath   string               `json:"manifestPath"`
	DurationMs     uint64               `json:"durationMs"`
	SourcesSynced  []string             `json:"sourcesSynced"`  // 非 nil
	SourcesSkipped []SkippedSource      `json:"sourcesSkipped"` // 非 nil
	PartialErrors  []string             `json:"partialErrors"`  // 非 nil；阶段级汇总（含 build 错误）
	Failures       []FetchFailureDetail `json:"failures"`       // 非 nil；逐文件抓取失败明细
}

// SyncStatus 是 Status 返回的 UI 状态横幅读模型（对齐 SyncStatus）。
type SyncStatus struct {
	UserDataPresent bool          `json:"userDataPresent"`
	Manifest        *Manifest     `json:"manifest"` // nil → null
	DataSizeMB      float64       `json:"dataSizeMb"`
	Archives        []ArchiveInfo `json:"archives"`     // 非 nil，最新在前
	RuntimeStats    ManifestStats `json:"runtimeStats"` // 当前可服务计数（首启即真实数）
}

// ArchiveInfo 是一个 bedrock-data.archive-<stamp> 归档目录的元数据（对齐 ArchiveInfo）。
type ArchiveInfo struct {
	Name      string  `json:"name"` // 目录名（非全路径）
	Path      string  `json:"path"`
	CreatedAt string  `json:"createdAt"` // 从目录名解析的时间戳串
	SizeMB    float64 `json:"sizeMb"`
}

// ResetReport 是 Reset 返回的归档信息（对齐 ResetReport）。
type ResetReport struct {
	ArchivedTo    string  `json:"archivedTo"`
	ArchiveSizeMB float64 `json:"archiveSizeMb"`
}

// Service 是 OTA 同步 Wails 服务。dataDir = 用户数据根（storage.UserDataDir），bedrock-data 落其下。
type Service struct {
	dataDir string
	hooks   BedrockHooks

	emitMu sync.Mutex
	emit   func(name string, payload any)

	cancelMu sync.Mutex
	cancel   context.CancelFunc // 当前 run 的取消句柄；nil = 无 run
}

// New 构造 OTA 同步服务。dataDir 为用户数据根（如 %AppData%\XK-Pane）；hooks 注入 bedrock 热切 + 计数。
func New(dataDir string, hooks BedrockHooks) *Service {
	return &Service{dataDir: dataDir, hooks: hooks}
}

// SetEmitter 注入进度事件发射回调（app 装配处传 mirrorEmit 双路：桌面 webview + Web 网关）。
func (s *Service) SetEmitter(fn func(name string, payload any)) {
	s.emitMu.Lock()
	s.emit = fn
	s.emitMu.Unlock()
}

// sink 构造一次 run 用的进度 sink，桥到 SetEmitter 注入的发射器（nil → 静默）。
func (s *Service) sink() ProgressSink {
	return NewProgressSink(func(p events.BedrockDocSyncProgressPayload) {
		s.emitMu.Lock()
		fn := s.emit
		s.emitMu.Unlock()
		if fn != nil {
			fn(events.BedrockDocSyncProgress, p)
		}
	})
}

func (s *Service) runtimeStats() ManifestStats {
	if s.hooks.RuntimeStats != nil {
		return s.hooks.RuntimeStats()
	}
	return ManifestStats{}
}

// bedrockDataDir 是 live 的 bedrock-data/ 目录绝对路径。
func (s *Service) bedrockDataDir() string { return filepath.Join(s.dataDir, "bedrock-data") }

// stagingDir 是 sync 期间的暂存目录（成功后原子换到 live）。
func (s *Service) stagingDir() string { return filepath.Join(s.dataDir, "bedrock-data.tmp") }

// ── 绑定方法（前端经 bindings 调用）─────────────────────────────────────────

// Sync 跑一次完整 OTA 同步。阻塞至完成，进度经 BedrockDocSyncProgress 事件流出。
// 同一时刻只允许一个 sync；并发调用返错。Cancel() 在阶段边界中止。
func (s *Service) Sync(ctx context.Context, opts SyncOptions) (SyncReport, error) {
	runCtx, cancel := context.WithCancel(ctx)
	s.cancelMu.Lock()
	if s.cancel != nil {
		s.cancelMu.Unlock()
		cancel()
		return SyncReport{}, fmt.Errorf("a sync is already in progress")
	}
	s.cancel = cancel
	s.cancelMu.Unlock()
	defer func() {
		s.cancelMu.Lock()
		s.cancel = nil
		s.cancelMu.Unlock()
		cancel()
	}()
	return s.runSync(runCtx, opts, s.sink())
}

// Cancel 取消当前正在跑的 sync（无 run 时 no-op）。
func (s *Service) Cancel() {
	s.cancelMu.Lock()
	c := s.cancel
	s.cancelMu.Unlock()
	if c != nil {
		c()
	}
}

// runSync 是同步主编排（对齐 service.rs::sync）。
func (s *Service) runSync(ctx context.Context, opts SyncOptions, progress ProgressSink) (SyncReport, error) {
	started := time.Now()

	// 解析有效源列表（空 ⇒ 默认 [mcguide, mconline]）。
	sources := opts.Sources
	if len(sources) == 0 {
		sources = []SourceSelector{SelMcguide, SelMconline}
	}
	srcStrs := make([]string, len(sources))
	for i, sel := range sources {
		srcStrs[i] = string(sel)
	}
	progress.Started(srcStrs)

	// 准备 staging。
	staging := s.stagingDir()
	if pathExists(staging) {
		if err := os.RemoveAll(staging); err != nil {
			return SyncReport{}, fmt.Errorf("clean staging: %w", err)
		}
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return SyncReport{}, fmt.Errorf("mkdir staging: %w", err)
	}

	cancelled := func() bool { return ctx.Err() != nil }
	abort := func() (SyncReport, error) {
		progress.Cancelled()
		_ = os.RemoveAll(staging)
		return SyncReport{}, fmt.Errorf("cancelled by user")
	}

	// 解析代理：显式 > 持久化 > env。
	explicit := opts.Proxy
	if explicit == nil {
		explicit = s.loadProxy()
	}
	proxy := ProxyConfigFromEnvOr(explicit)

	// 解析源覆盖：显式 > 持久化 sources.json > 默认（effective_* 兜底）。
	var sourcesCfg SyncSourcesConfig
	if opts.SourcesConfig != nil {
		sourcesCfg = *opts.SourcesConfig
	} else if loaded := s.loadSources(); loaded != nil {
		sourcesCfg = *loaded
	}
	mcguideRemote := sourcesCfg.EffectiveMcguideRemote()
	mcguideBranch := sourcesCfg.EffectiveMcguideBranch()
	mconlineRemote := sourcesCfg.EffectiveMconlineRemote()
	mconlineBranch := sourcesCfg.EffectiveMconlineBranch()

	concurrency := 4
	if opts.Concurrency != nil {
		concurrency = int(*opts.Concurrency)
	}
	if concurrency < 1 {
		concurrency = 1
	} else if concurrency > 16 {
		concurrency = 16
	}

	sourcesMap := map[string]SourceInfo{}
	synced := []string{}
	skipped := []SkippedSource{}
	partial := []string{}
	failures := []FetchFailureDetail{}

	// ===== Wiki 源，按 (remote, branch) 分桶 =====
	// 同 remote 的 mcguide/mconline 合并为一次 gitea_sync（单次列树 + 并发拉 blob）。
	type wikiKey struct{ remote, branch string }
	groups := map[wikiKey][]string{}
	for _, src := range sources {
		switch src {
		case SelMcguide:
			k := wikiKey{mcguideRemote, mcguideBranch}
			groups[k] = append(groups[k], string(src))
		case SelMconline:
			k := wikiKey{mconlineRemote, mconlineBranch}
			groups[k] = append(groups[k], string(src))
		case SelMcdocs:
			// 下方处理
		}
	}
	groupKeys := make([]wikiKey, 0, len(groups))
	for k := range groups {
		groupKeys = append(groupKeys, k)
	}
	sort.Slice(groupKeys, func(i, j int) bool {
		if groupKeys[i].remote != groupKeys[j].remote {
			return groupKeys[i].remote < groupKeys[j].remote
		}
		return groupKeys[i].branch < groupKeys[j].branch
	})

	docsDir := filepath.Join(staging, "data", "docs")
	for _, k := range groupKeys {
		if cancelled() {
			return abort()
		}
		subdirs := groups[k]
		remote, branch := k.remote, k.branch
		groupLabel := strings.Join(subdirs, "+")
		sha := ""

		apiBase, rawBase, owner, repo, perr := parseGiteaURL(remote)
		if perr != nil {
			progress.ErrorLog(fmt.Sprintf("wiki[%s]: %s is not a Gitea endpoint (%v); embedded fallback", groupLabel, remote, perr))
			for _, sub := range subdirs {
				sourcesMap[sub] = EmbeddedSource()
				skipped = append(skipped, SkippedSource{Source: sub, Reason: fmt.Sprintf("not a Gitea URL: %v", perr)})
			}
			continue
		}
		progress.Info(fmt.Sprintf("wiki[%s]: gitea_sync %s/%s@%s", groupLabel, owner, repo, branch))

		client, cerr := proxy.HTTPClient()
		var report GiteaSyncReport
		var serr error
		if cerr != nil {
			serr = cerr
		} else {
			report, serr = giteaSync(ctx, client, GiteaSyncParams{
				SourceName:   fmt.Sprintf("wiki[%s]", groupLabel),
				APIBase:      apiBase,
				RawBase:      rawBase,
				Owner:        owner,
				Repo:         repo,
				Branch:       branch,
				Subdirs:      subdirs,
				ExtWhitelist: []string{"md", "json"},
				TargetDir:    docsDir,
				Concurrency:  concurrency,
			}, progress)
		}
		if serr != nil {
			progress.ErrorLog(fmt.Sprintf("wiki[%s]: gitea_sync failed (%v); embedded fallback", groupLabel, serr))
			// 清理任何部分文件，保持 staging 一致。
			for _, sub := range subdirs {
				_ = os.RemoveAll(filepath.Join(docsDir, sub))
				sourcesMap[sub] = EmbeddedSource()
				skipped = append(skipped, SkippedSource{Source: sub, Reason: fmt.Sprintf("wiki gitea_sync failed: %v", serr)})
			}
			continue
		}
		sha = report.TreeSHA
		if len(report.Failed) > 0 {
			partial = append(partial, fmt.Sprintf("wiki[%s] gitea partial: %d files failed", groupLabel, len(report.Failed)))
			srcLabel := fmt.Sprintf("wiki[%s]", groupLabel)
			for _, f := range report.Failed {
				failures = append(failures, FetchFailureDetail{Source: srcLabel, Path: f.Path, Error: f.Error})
			}
		}

		// 逐 subdir 写 manifest：落了文件 → Gitea，否则 Embedded。
		for _, sub := range subdirs {
			dest := filepath.Join(docsDir, sub)
			if dirNonEmpty(dest) {
				sourcesMap[sub] = GiteaSource(remote, branch, sha, iso8601Now())
				synced = append(synced, sub)
			} else {
				progress.Warn(fmt.Sprintf("wiki source produced no files for subdir '%s'; falling back to embedded", sub))
				sourcesMap[sub] = EmbeddedSource()
				skipped = append(skipped, SkippedSource{Source: sub, Reason: fmt.Sprintf("wiki source has no files under '%s'", sub)})
			}
		}
	}

	// ===== mcdocs（VuePress http_sync）=====
	for _, src := range sources {
		if cancelled() {
			return abort()
		}
		switch src {
		case SelMcguide, SelMconline:
			// 上方已处理
		case SelMcdocs:
			// 解析序：显式 opts.McdocsBaseURL > 持久化 > 官方 URL。
			url := sourcesCfg.EffectiveMcdocsBaseURL()
			if strings.TrimSpace(opts.McdocsBaseURL) != "" {
				url = opts.McdocsBaseURL
			}
			progress.Info(fmt.Sprintf("mcdocs: attempting VuePress http_sync from %s", url))
			target := filepath.Join(docsDir, "mcdocs")

			client, cerr := proxy.HTTPClient()
			var report HTTPSyncReport
			var herr error
			if cerr != nil {
				herr = cerr
			} else {
				report, herr = httpSync(ctx, client, HTTPSyncParams{
					SourceName:  "mcdocs",
					BaseURL:     url,
					TargetDir:   target,
					Concurrency: concurrency,
					Mode:        HTTPSyncModeVuepress,
				}, progress)
			}
			if herr != nil {
				progress.ErrorLog(fmt.Sprintf("mcdocs http_sync failed: %v", herr))
				sourcesMap["mcdocs"] = EmbeddedSource()
				skipped = append(skipped, SkippedSource{Source: "mcdocs", Reason: fmt.Sprintf("http_sync failed: %v", herr)})
				continue
			}
			sourcesMap["mcdocs"] = HTTPSource(url, report.Fetched, iso8601Now())
			synced = append(synced, "mcdocs")
			if len(report.Failed) > 0 {
				partial = append(partial, fmt.Sprintf("mcdocs http_sync had %d failures", len(report.Failed)))
				for _, f := range report.Failed {
					failures = append(failures, FetchFailureDetail{Source: "mcdocs", Path: f.Path, Error: f.Error})
				}
			}
		}
	}

	if cancelled() {
		return abort()
	}

	// 条件构建：仅当 mcdocs 落进 staging 才建 api-index。
	mcdocsDir := filepath.Join(docsDir, "mcdocs")
	if isDir(mcdocsDir) {
		progress.Info("running build_api_index")
		if r, err := buildAPIIndex(mcdocsDir, filepath.Join(staging, "data", "api-index.json")); err != nil {
			progress.Warn(fmt.Sprintf("build_api_index failed: %v", err))
			partial = append(partial, fmt.Sprintf("build_api_index: %v", err))
		} else {
			progress.BuildIndex("api", r.Total)
		}
	} else {
		progress.Info("skipping build_api_index (no mcdocs in staging)")
	}

	// 条件构建：仅当 demo 树落进 staging 才建 templates-index。
	demoDir := filepath.Join(staging, "data", "code", "3.6Demo", "6-1DemoMod")
	if isDir(demoDir) {
		progress.Info("running build_templates_index")
		if r, err := buildTemplatesIndex(demoDir, filepath.Join(staging, "data", "templates-index.json")); err != nil {
			progress.Warn(fmt.Sprintf("build_templates_index failed: %v", err))
			partial = append(partial, fmt.Sprintf("build_templates_index: %v", err))
		} else {
			progress.BuildIndex("templates", r.Total)
		}
	}

	progress.Validating()

	// 原子换目录：live → .bak，staging → live，删 .bak。
	bedrockData := s.bedrockDataDir()
	backup := filepath.Join(s.dataDir, "bedrock-data.bak")
	if pathExists(backup) {
		_ = os.RemoveAll(backup)
	}
	if pathExists(bedrockData) {
		if err := os.Rename(bedrockData, backup); err != nil {
			return SyncReport{}, fmt.Errorf("backup existing bedrock-data: %w", err)
		}
	}
	if err := os.Rename(staging, bedrockData); err != nil {
		// 回滚：尝试恢复备份。
		if pathExists(backup) {
			_ = os.Rename(backup, bedrockData)
		}
		return SyncReport{}, fmt.Errorf("atomic swap failed: %v", err)
	}
	if pathExists(backup) {
		_ = os.RemoveAll(backup)
	}

	// 先让 bedrock 查询层对新树失效缓存，再算 runtime stats（否则 loader 还指着换前的源）。
	if s.hooks.SetDataRoot != nil {
		s.hooks.SetDataRoot(bedrockData)
	}

	// manifest.stats 反映 loader 当前真正能服务的（用户数据 ∪ 内嵌兜底），而非本次写盘量
	// （模板/组件/未刷新子树只在内嵌基线里；mcdocs 失败时 loader 仍服务旧 api-index）。
	stats := s.runtimeStats()
	manifest := Manifest{
		SchemaVersion: 1,
		SyncedAt:      iso8601Now(),
		Sources:       sourcesMap,
		Stats:         stats,
	}
	manifestPath := filepath.Join(bedrockData, "manifest.json")
	data, err := marshalIndentNoEscape(manifest)
	if err != nil {
		return SyncReport{}, fmt.Errorf("serialize manifest: %w", err)
	}
	if err := filex.WriteAtomic(manifestPath, data, 0o644); err != nil {
		return SyncReport{}, fmt.Errorf("write manifest.json: %w", err)
	}

	durationMs := uint64(time.Since(started).Milliseconds())
	progress.Completed(manifestPath, durationMs)

	return SyncReport{
		ManifestPath:   manifestPath,
		DurationMs:     durationMs,
		SourcesSynced:  synced,
		SourcesSkipped: skipped,
		PartialErrors:  partial,
		Failures:       failures,
	}, nil
}

// Status 返回 bedrock-data/ 当前状态供 UI 横幅（对齐 status）。runtime_stats 永远反映 loader
// 此刻可服务的真实数（即便从未跑过 OTA）。
func (s *Service) Status() (SyncStatus, error) {
	bedrockData := s.bedrockDataDir()
	userDataPresent := pathExists(bedrockData)
	var manifest *Manifest
	manifestPath := filepath.Join(bedrockData, "manifest.json")
	if pathExists(manifestPath) {
		if b, err := os.ReadFile(manifestPath); err == nil {
			var m Manifest
			if json.Unmarshal(b, &m) == nil {
				manifest = &m
			}
		}
	}
	return SyncStatus{
		UserDataPresent: userDataPresent,
		Manifest:        manifest,
		DataSizeMB:      dirSizeMB(bedrockData),
		Archives:        s.listArchives(),
		RuntimeStats:    s.runtimeStats(),
	}, nil
}

// Reset 把 bedrock-data/ 移到 bedrock-data.archive-<stamp>/ 并清 SetDataRoot 回退内嵌基线
// （对齐 reset）。最多保留 5 个归档。
func (s *Service) Reset() (ResetReport, error) {
	bedrockData := s.bedrockDataDir()
	if !pathExists(bedrockData) {
		return ResetReport{}, fmt.Errorf("no user data to reset")
	}
	stamp := compactStampNow()
	archive := filepath.Join(s.dataDir, "bedrock-data.archive-"+stamp)
	if err := os.Rename(bedrockData, archive); err != nil {
		return ResetReport{}, fmt.Errorf("rename bedrock-data to archive: %w", err)
	}
	if s.hooks.SetDataRoot != nil {
		s.hooks.SetDataRoot("")
	}
	archiveSizeMB := dirSizeMB(archive)
	s.pruneOldArchives(5)
	return ResetReport{ArchivedTo: archive, ArchiveSizeMB: archiveSizeMB}, nil
}

// ListArchives 列出历史 bedrock-data.archive-* 目录（最新在前，对齐 list_archives）。
func (s *Service) ListArchives() ([]ArchiveInfo, error) {
	return s.listArchives(), nil
}

func (s *Service) listArchives() []ArchiveInfo {
	out := []ArchiveInfo{}
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		stamp, ok := strings.CutPrefix(name, "bedrock-data.archive-")
		if !ok {
			continue
		}
		p := filepath.Join(s.dataDir, name)
		out = append(out, ArchiveInfo{Name: name, Path: p, CreatedAt: stamp, SizeMB: dirSizeMB(p)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// SetProxy 探测代理（HEAD github.com），仅在探测成功时持久化到 proxy.json（对齐 set_proxy）。
// 探测失败返回 ok=false 的结果且不写盘（不视为 error）。
func (s *Service) SetProxy(ctx context.Context, proxy ProxyConfig) (ProbeResult, error) {
	probe, perr := proxy.Probe(ctx)
	if perr != nil {
		probe = ProbeResult{OK: false, StatusCode: 0, LatencyMs: 0}
	}
	if probe.OK {
		path := s.proxyConfigPath()
		if parent := filepath.Dir(path); parent != "" {
			_ = os.MkdirAll(parent, 0o755)
		}
		data, err := marshalIndentNoEscape(proxy)
		if err != nil {
			return probe, fmt.Errorf("serialize proxy cfg: %w", err)
		}
		if err := filex.WriteAtomic(path, data, 0o644); err != nil {
			return probe, fmt.Errorf("write proxy.json: %w", err)
		}
	}
	return probe, nil
}

// GetProxy 读持久化代理配置（无则零值；对齐 load_proxy 但返值而非 Option）。
func (s *Service) GetProxy() ProxyConfig {
	if p := s.loadProxy(); p != nil {
		return *p
	}
	return ProxyConfig{}
}

// SetSources 持久化 per-source remote/branch 覆盖到 sources.json（无探测；对齐 set_sources）。
func (s *Service) SetSources(cfg SyncSourcesConfig) error {
	path := s.sourcesConfigPath()
	if parent := filepath.Dir(path); parent != "" {
		_ = os.MkdirAll(parent, 0o755)
	}
	data, err := marshalIndentNoEscape(cfg)
	if err != nil {
		return fmt.Errorf("serialize sources cfg: %w", err)
	}
	if err := filex.WriteAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("write sources.json: %w", err)
	}
	return nil
}

// GetSources 读持久化源覆盖（无则零值；对齐 load_sources 但返值而非 Option）。
func (s *Service) GetSources() SyncSourcesConfig {
	if c := s.loadSources(); c != nil {
		return *c
	}
	return SyncSourcesConfig{}
}

// ── 内部 helper ─────────────────────────────────────────────────────────

func (s *Service) loadProxy() *ProxyConfig {
	path := s.proxyConfigPath()
	if !pathExists(path) {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var p ProxyConfig
	if json.Unmarshal(b, &p) != nil {
		return nil
	}
	return &p
}

func (s *Service) loadSources() *SyncSourcesConfig {
	path := s.sourcesConfigPath()
	if !pathExists(path) {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c SyncSourcesConfig
	if json.Unmarshal(b, &c) != nil {
		return nil
	}
	return &c
}

// proxyConfigPath / sourcesConfigPath：bedrock-data/ 存在时落其内（OTA 树拥有该文件），否则落
// 数据根旁的 bedrock-data.<name>.json（让设置在首次 sync 前就能存活；对齐 *_config_path）。
func (s *Service) proxyConfigPath() string {
	bd := s.bedrockDataDir()
	if pathExists(bd) {
		return filepath.Join(bd, "proxy.json")
	}
	return filepath.Join(s.dataDir, "bedrock-data.proxy.json")
}

func (s *Service) sourcesConfigPath() string {
	bd := s.bedrockDataDir()
	if pathExists(bd) {
		return filepath.Join(bd, "sources.json")
	}
	return filepath.Join(s.dataDir, "bedrock-data.sources.json")
}

func (s *Service) pruneOldArchives(keep int) {
	archives := s.listArchives()
	for i := keep; i < len(archives); i++ {
		_ = os.RemoveAll(archives[i].Path)
	}
}

// ── 纯函数 helper ───────────────────────────────────────────────────────

// parseGiteaURL 把 Gitea 风格 HTTPS remote 拆成 (apiBase, rawBase, owner, repo)（对齐
// parse_gitea_url）。**github.com 被刻意拒绝**（无 Gitea REST API）→ 调用方回退内嵌基线。
func parseGiteaURL(rawURL string) (apiBase, rawBase, owner, repo string, err error) {
	trimmed := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	trimmed = strings.TrimSuffix(trimmed, ".git")
	schemeEnd := strings.Index(trimmed, "://")
	if schemeEnd < 0 {
		return "", "", "", "", fmt.Errorf("missing scheme")
	}
	scheme := trimmed[:schemeEnd+3] // 含 "://"
	rest := trimmed[schemeEnd+3:]
	parts := strings.SplitN(rest, "/", 3)
	host := ""
	if len(parts) >= 1 {
		host = parts[0]
	}
	if host == "" {
		return "", "", "", "", fmt.Errorf("missing host segment")
	}
	if strings.EqualFold(host, "github.com") {
		return "", "", "", "", fmt.Errorf("github.com is not a Gitea instance")
	}
	if len(parts) < 2 || parts[1] == "" {
		return "", "", "", "", fmt.Errorf("missing owner segment")
	}
	owner = parts[1]
	if len(parts) < 3 || parts[2] == "" {
		return "", "", "", "", fmt.Errorf("missing repo segment")
	}
	repo = strings.TrimSuffix(strings.TrimRight(parts[2], "/"), ".git")
	rawBase = scheme + host
	apiBase = rawBase + "/api/v1"
	return apiBase, rawBase, owner, repo, nil
}

// iso8601Now 返回 UTC ISO 8601 秒精度时间戳（对齐 iso8601_now：%Y-%m-%dT%H:%M:%SZ）。
func iso8601Now() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

// compactStampNow 返回 UTC 紧凑时间戳（对齐 compact_stamp_now：%Y%m%d-%H%M%S）。
func compactStampNow() string { return time.Now().UTC().Format("20060102-150405") }

// dirSizeMB 递归求 p 下文件字节总和（MB），缺失/出错路径返 0（对齐 dir_size_mb）。
func dirSizeMB(p string) float64 { return float64(dirSizeBytes(p)) / (1024.0 * 1024.0) }

// dirSizeBytes 递归累加常规文件大小（os.Lstat 不跟随符号链接，对齐 symlink_metadata）。
func dirSizeBytes(p string) uint64 {
	entries, err := os.ReadDir(p)
	if err != nil {
		return 0
	}
	var total uint64
	for _, e := range entries {
		full := filepath.Join(p, e.Name())
		fi, err := os.Lstat(full)
		if err != nil {
			continue
		}
		switch mode := fi.Mode(); {
		case mode.IsRegular():
			total += uint64(fi.Size())
		case mode.IsDir():
			total += dirSizeBytes(full)
		}
	}
	return total
}

// dirNonEmpty 判目录存在且至少有一个子项（对齐 dest.is_dir() && read_dir().next().is_some()）。
func dirNonEmpty(d string) bool {
	entries, err := os.ReadDir(d)
	return err == nil && len(entries) > 0
}
