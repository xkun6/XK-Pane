package storage

// models_mcpe.go — MC PE 开发包持久化模型（移植 cc-pane models/mc_pe.rs，P6e）。
//
// 一个 MCPackage = 一个网易《我的世界》中国版组件项目 + 一组运行配置。嵌套的 World /
// DebugOptions / IncludedModDirs 用 GORM serializer:json 存为 JSON 列（保持与前端 camelCase
// 契约一致的嵌套结构）。时间戳用 Go 原生 time.Time（GORM 自管，JSON 序列化为 RFC3339），
// 不沿用 cc-pane 的 RFC3339 文本列（与本仓库其它模型一致）。

import (
	"math/rand/v2"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

func init() {
	// Clearable:false —— MC 包是用户精心配置的项目/世界设置，不进设置页一键清空（同 projects 档）。
	AllModels = append(AllModels,
		ModelDescriptor{Model: &MCPackage{}, TableName: "mc_packages", LabelKey: "mcPackages", Clearable: false},
	)
}

// MCGameMode 游戏模式（序列化值小写，对齐前端 GAME_MODES）。
type MCGameMode string

const (
	MCGameModeSurvival MCGameMode = "survival"
	MCGameModeCreative MCGameMode = "creative"
)

// Code 返回 level.dat GameType 值（Survival=0 / Creative=1）。
func (m MCGameMode) Code() int32 {
	if m == MCGameModeCreative {
		return 1
	}
	return 0
}

// MCLevelType 世界类型。
type MCLevelType string

const (
	MCLevelTypeDefault MCLevelType = "default"
	MCLevelTypeFlat    MCLevelType = "flat"
)

// Code 返回 level.dat Generator 值（Default=1 / Flat=2）。
func (t MCLevelType) Code() int32 {
	if t == MCLevelTypeFlat {
		return 2
	}
	return 1
}

// MCWorldConfig 世界配置。
type MCWorldConfig struct {
	FolderName      string      `json:"folderName"`
	Seed            int64       `json:"seed"`
	UserName        string      `json:"userName"`
	GameMode        MCGameMode  `json:"gameMode"`
	LevelType       MCLevelType `json:"levelType"`
	EnableCheats    bool        `json:"enableCheats"`
	KeepInventory   bool        `json:"keepInventory"`
	// ShowCoordinates 显示坐标（HUD 左上角）。用指针：存量包的 World JSON 没有此字段 → 反序列化为 nil，
	// 应用时按 true 处理，让“默认显示坐标”对历史包也生效；新包在 NewMCPackage 里显式置 true。
	ShowCoordinates *bool       `json:"showCoordinates"`
	DoDaylightCycle bool        `json:"doDaylightCycle"`
	DoWeatherCycle  bool        `json:"doWeatherCycle"`
	// SourceDir 可选存档源目录：启动时若 minecraftWorlds/<folderName> 不存在，从此目录复制副本（不改原图）。
	SourceDir string `json:"sourceDir,omitempty"`
	// ResetOnLaunch 每次启动前清空副本再从 SourceDir 重新复制（临时调试用，默认 false）。
	ResetOnLaunch bool `json:"resetOnLaunch"`
}

// MCDebugOptions 调试热键 + 桥开关。
type MCDebugOptions struct {
	ReloadKey        int32  `json:"reloadKey"`
	ReloadWorldKey   int32  `json:"reloadWorldKey"`
	ReloadAddonKey   *int32 `json:"reloadAddonKey,omitempty"`
	ReloadShadersKey *int32 `json:"reloadShadersKey,omitempty"`
	ReloadKeyGlobal  bool   `json:"reloadKeyGlobal"`
	// IncludeTestMod 启动前把桥模组解压进 behavior_packs 并注册到 world json（默认 true）；
	// 关掉时同时跳过 MCDEV_DEBUG_IPC_PORT 注入。
	IncludeTestMod bool `json:"includeTestMod"`
}

// MCPackage MC PE 开发包（GORM 模型 + 前端契约）。
type MCPackage struct {
	ID                 string         `gorm:"primaryKey;size:36" json:"id"`
	Name               string         `gorm:"type:text;not null" json:"name"`
	ProjectPath        string         `gorm:"type:text;not null" json:"projectPath"`
	GameExecutablePath string         `gorm:"type:text" json:"gameExecutablePath,omitempty"`
	IncludedModDirs    []string       `gorm:"type:text;serializer:json" json:"includedModDirs"`
	World              MCWorldConfig  `gorm:"type:text;serializer:json" json:"world"`
	DebugOptions       MCDebugOptions `gorm:"type:text;serializer:json" json:"debugOptions"`
	// SortOrder 列表显示顺序（@dnd-kit 拖拽写入，0 为默认；存量包全为 0 时按 CreatedAt 兜底排序）。
	SortOrder int       `gorm:"not null;default:0" json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (MCPackage) TableName() string { return "mc_packages" }

// NewMCPackage 用项目路径构造默认配置的新包（移植 MCPackage::new）。
func NewMCPackage(projectPath, name string) MCPackage {
	resolved := name
	if resolved == "" {
		resolved = filepath.Base(projectPath)
		if resolved == "" || resolved == "." || resolved == string(filepath.Separator) {
			resolved = "MC PE Package"
		}
	}
	showCoords := true // 新包默认显示坐标
	return MCPackage{
		ID:              uuid.NewString(),
		Name:            resolved,
		ProjectPath:     projectPath,
		IncludedModDirs: []string{},
		World: MCWorldConfig{
			FolderName:      uuid.NewString(),
			Seed:            int64(rand.Uint64()), //nolint:gosec // 世界种子无需密码学随机
			UserName:        "DevOps",
			GameMode:        MCGameModeCreative,
			LevelType:       MCLevelTypeFlat,
			EnableCheats:    true,
			KeepInventory:   true,
			ShowCoordinates: &showCoords,
			DoDaylightCycle: false,
			DoWeatherCycle:  false,
			ResetOnLaunch:   false,
		},
		DebugOptions: MCDebugOptions{
			ReloadKey:       82, // R
			ReloadWorldKey:  96, // Numpad0
			ReloadKeyGlobal: false,
			IncludeTestMod:  true,
		},
		// CreatedAt / UpdatedAt 由 GORM 在 Create 时自管。
	}
}

// MCRunSessionInfo 一次启动的返回信息（PID 为 nil 表示进程未提供有效 pid）。
type MCRunSessionInfo struct {
	RunID     string `json:"runId"`
	PackageID string `json:"packageId"`
	PID       *int   `json:"pid,omitempty"`
}

// MCRunSummary 一个活跃 run 的摘要（get_mc_pe_logs 多客户端分节 / execute·reload 消歧提示用）。
type MCRunSummary struct {
	RunID       string `json:"runId"`
	PackageID   string `json:"packageId"`
	PackageName string `json:"packageName"`
	PID         *int   `json:"pid,omitempty"`
}

// McdkBridgeStatus MCDK 调试桥状态（前端运行面板轮询）。未连接时 RemoteAddr/ConnectedAtMs 为 null。
type McdkBridgeStatus struct {
	Port          int     `json:"port"`
	Connected     bool    `json:"connected"`
	RemoteAddr    *string `json:"remoteAddr"`
	ConnectedAtMs *int64  `json:"connectedAtMs"`
}
