package localhistory

import (
	"encoding/json"
	"os"
	"path/filepath"

	"xkpane/internal/utils/filex"
)

const configFile = "config.json"

// HistoryConfig 是每项目本地历史配置（对齐 cc-pane HistoryConfig）。
//
// 偏离 cc-pane：cc-pane 存 `.ccpanes/config.toml`（TOML）；本仓存 `.xkpanes/config.json`
// （JSON + filex 原子写）——与 XK-Pane 的 storage.json 一致，免引 TOML 依赖。字段语义不变。
type HistoryConfig struct {
	Enabled             bool     `json:"enabled"`
	IgnorePatterns      []string `json:"ignorePatterns"`
	MaxVersionsPerFile  int      `json:"maxVersionsPerFile"`
	MaxAgeDays          int      `json:"maxAgeDays"`
	MaxFileSize         int64    `json:"maxFileSize"`         // 单文件大小上限（字节），超过不记历史
	MaxTotalSize        int64    `json:"maxTotalSize"`        // 项目级总存储上限（字节）
	MinSaveIntervalSecs int64    `json:"minSaveIntervalSecs"` // 同文件同分支最小保存间隔（秒）
}

// DefaultHistoryConfig 返回默认配置（ignore 把 .ccpanes 换成 .xkpanes）。
//
// **有意偏离 cc-pane**：MinSaveIntervalSecs 默认 0（cc-pane 是 300=5 分钟）。本地历史的价值在「每次
// 修改都能回看」，5 分钟节流会让同文件 5 分钟内只存一版、看不到中间编辑——失去 IDE Local History 语义。
// 改 0 = 每个「内容变化的保存」都记一版；500ms debounce 已把一次保存的多个事件合一、hash 去重已跳过
// 内容未变的保存，故不会刷屏。存储量由 MaxVersionsPerFile/MaxAgeDays + 清理兜底。用户可在配置面板按需调高。
func DefaultHistoryConfig() HistoryConfig {
	return HistoryConfig{
		Enabled: true,
		IgnorePatterns: []string{
			"node_modules/**", ".git/**", "target/**", "dist/**", "build/**",
			"*.log", "*.lock", ".xkpanes/**",
		},
		MaxVersionsPerFile:  50,
		MaxAgeDays:          30,
		MaxFileSize:         5 * 1024 * 1024,
		MaxTotalSize:        500 * 1024 * 1024,
		MinSaveIntervalSecs: 0,
	}
}

func (s *Store) configPath() string {
	return filepath.Join(s.projectPath, dirName, configFile)
}

// ReadConfig 读取项目历史配置；文件不存在或字段缺失则取默认值（缺失字段保留默认，
// 等价 cc-pane 的 serde default 行为）。
func (s *Store) ReadConfig() (HistoryConfig, error) {
	cfg := DefaultHistoryConfig()
	data, err := os.ReadFile(s.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	// 在默认值之上反序列化：present 字段覆盖，缺失字段保留默认。
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultHistoryConfig(), err
	}
	return cfg, nil
}

// SaveConfig 原子写项目历史配置（JSON）。
func (s *Store) SaveConfig(cfg HistoryConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return filex.WriteAtomic(s.configPath(), data, 0o644)
}
