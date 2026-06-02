// Package bedrockdocsync 是 Bedrock 知识库的 OTA 同步子系统（对齐 cc-pane bedrock_doc_sync）：
// 从 Gitea（git.mcplugin.top 的 netease-bedrock-wiki 镜像）+ VuePress（mc.163.com mcdocs）爬最新
// 文档，构建 *-index.json，原子换目录到 bedrock-data/，并经 bedrock.SetDataRoot 让查询层热切换。
// 全程无 git2（Gitea API + 纯 HTTP），代理走 httpx.NewClient。
//
// 本文件 = 快照元数据与同步入参模型（对齐 manifest.rs）。
//
// 与 cc-pane 的偏离：JSON tag 用 **camelCase**（XK 约定，跨 Wails 到前端 + 写 XK 自己的
// bedrock-data，不与 cc-pane 共享磁盘格式，故无需 snake_case 字节兼容）。
package bedrockdocsync

import "strings"

// 官方上游默认（直连，无 cada-mcp 中介）。effective_* 与 service.sync 据此对齐。
const (
	DefaultWikiRemote    = "https://git.mcplugin.top/xiaokun/netease-bedrock-wiki"
	DefaultWikiBranch    = "main"
	DefaultMcdocsBaseURL = "https://mc.163.com/dev/mcmanual/mc-dev/mcdocs/"
)

// Manifest 是当前快照元数据，写到 bedrock-data/manifest.json。
type Manifest struct {
	SchemaVersion uint32                `json:"schemaVersion"`
	SyncedAt      string                `json:"syncedAt"` // ISO 8601 UTC
	Sources       map[string]SourceInfo `json:"sources"`  // 键：mcguide / mconline / mcdocs
	Stats         ManifestStats         `json:"stats"`
}

// SourceInfo 是单个来源的溯源信息。Kind 判别（gitea/http/embedded）；按 Kind 取用对应字段
// （扁平化 cc-pane 的 tagged enum，omitempty 让不相关字段不出现）。
type SourceInfo struct {
	Kind      string `json:"kind"` // "gitea" | "http" | "embedded"
	Remote    string `json:"remote,omitempty"`
	Branch    string `json:"branch,omitempty"`
	TreeSHA   string `json:"treeSha,omitempty"`
	BaseURL   string `json:"baseUrl,omitempty"`
	FileCount uint32 `json:"fileCount,omitempty"`
	FetchedAt string `json:"fetchedAt,omitempty"`
}

func GiteaSource(remote, branch, treeSHA, fetchedAt string) SourceInfo {
	return SourceInfo{Kind: "gitea", Remote: remote, Branch: branch, TreeSHA: treeSHA, FetchedAt: fetchedAt}
}

func HTTPSource(baseURL string, fileCount uint32, fetchedAt string) SourceInfo {
	return SourceInfo{Kind: "http", BaseURL: baseURL, FileCount: fileCount, FetchedAt: fetchedAt}
}

func EmbeddedSource() SourceInfo { return SourceInfo{Kind: "embedded"} }

// ManifestStats 是给 UI 数据状态横幅的头条计数。
type ManifestStats struct {
	APIEntries      uint32 `json:"apiEntries"`
	TemplateEntries uint32 `json:"templateEntries"`
	GuideFiles      uint32 `json:"guideFiles"`
}

// SourceSelector 是来源标识（同时用于 SyncOptions.Sources 与进度事件）。
type SourceSelector string

const (
	SelMcguide  SourceSelector = "mcguide"
	SelMconline SourceSelector = "mconline"
	SelMcdocs   SourceSelector = "mcdocs"
)

// SyncSourcesConfig 是各来源 remote / base URL 覆盖；空字符串等同于“用官方默认”。
// 持久化到 bedrock-data/sources.json。对齐 manifest.rs:136-147 + effective_*。
type SyncSourcesConfig struct {
	McguideRemote  string `json:"mcguideRemote,omitempty"`
	McguideBranch  string `json:"mcguideBranch,omitempty"`
	MconlineRemote string `json:"mconlineRemote,omitempty"`
	MconlineBranch string `json:"mconlineBranch,omitempty"`
	McdocsBaseURL  string `json:"mcdocsBaseUrl,omitempty"`
}

func nonEmptyOr(s, def string) string {
	if t := strings.TrimSpace(s); t != "" {
		return t
	}
	return def
}

func (c SyncSourcesConfig) EffectiveMcguideRemote() string {
	return nonEmptyOr(c.McguideRemote, DefaultWikiRemote)
}
func (c SyncSourcesConfig) EffectiveMcguideBranch() string {
	return nonEmptyOr(c.McguideBranch, DefaultWikiBranch)
}
func (c SyncSourcesConfig) EffectiveMconlineRemote() string {
	return nonEmptyOr(c.MconlineRemote, DefaultWikiRemote)
}
func (c SyncSourcesConfig) EffectiveMconlineBranch() string {
	return nonEmptyOr(c.MconlineBranch, DefaultWikiBranch)
}
func (c SyncSourcesConfig) EffectiveMcdocsBaseURL() string {
	return nonEmptyOr(c.McdocsBaseURL, DefaultMcdocsBaseURL)
}

// SyncOptions 是 Service.Sync 的入参。对齐 manifest.rs:102-123。
type SyncOptions struct {
	// Sources 要同步的来源；空 ⇒ 默认 [mcguide, mconline, mcdocs]。
	Sources []SourceSelector `json:"sources,omitempty"`
	// Concurrency 引擎内并发；nil ⇒ 4，clamp 到 [1,16]。
	Concurrency *uint32 `json:"concurrency,omitempty"`
	// Proxy 显式代理覆盖；nil ⇒ 持久化代理或环境变量。
	Proxy *ProxyConfig `json:"proxy,omitempty"`
	// McdocsBaseURL 可选 mcdocs base URL 覆盖。
	McdocsBaseURL string `json:"mcdocsBaseUrl,omitempty"`
	// SourcesConfig 可选 per-source remote/branch 覆盖；nil ⇒ 读持久化 sources.json。
	SourcesConfig *SyncSourcesConfig `json:"sourcesConfig,omitempty"`
}
