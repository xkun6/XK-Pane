package mcpe

// pack.go — 网易行为包/资源包 manifest.json 解析。忠实移植 pack.rs。
//
// manifest.json 结构：
//
//	{
//	  "header": { "uuid": "...", "version": [1,0,0], ... },
//	  "modules": [ {"type":"data"}, {"type":"resources"} ]   // data=行为包 / resources=资源包
//	}

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PackType 包类型（序列化值小写，对齐 pack.rs serde rename_all="lowercase"）。
type PackType string

const (
	PackBehavior PackType = "behavior"
	PackResource PackType = "resource"
)

// PackInfo 一个解析出的包。Path 是含 manifest.json 的目录（symlink 的源 target）。
type PackInfo struct {
	Type    PackType
	UUID    string
	Version string // "1.0.0"
	Path    string
}

type manifestDoc struct {
	Header *struct {
		UUID    string        `json:"uuid"`
		Version []json.Number `json:"version"`
	} `json:"header"`
	Modules []struct {
		Type string `json:"type"`
	} `json:"modules"`
}

// parsePack 解析 root 下最多深度 2 的所有 manifest.json，按类型分组追加进 out。
//
// 扫描策略：深度 0 = root/manifest.json；深度 1 = root/<child>/manifest.json；
// 深度 2 = 仅当 <child> 自己不是合法包时，再尝试 root/<child>/<grandchild>/manifest.json
// （支持 BDS 存档布局 world/{behavior_packs,resource_packs}/<pack>/manifest.json）。
// 深度 1 命中即跳过下钻，避免对正常项目结构多扫。返回是否找到至少一个合法包。
func parsePack(root string, out map[PackType][]PackInfo) bool {
	if !isDir(root) {
		return false
	}
	found := false
	if info, ok := parseManifest(filepath.Join(root, "manifest.json")); ok {
		out[info.Type] = append(out[info.Type], info)
		found = true
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return found
	}
	for _, e := range entries {
		child := filepath.Join(root, e.Name())
		if !isDir(child) {
			continue
		}
		if info, ok := parseManifest(filepath.Join(child, "manifest.json")); ok {
			out[info.Type] = append(out[info.Type], info)
			found = true
			continue // 深度 1 命中不下钻
		}
		subEntries, err := os.ReadDir(child)
		if err != nil {
			continue
		}
		for _, s := range subEntries {
			grand := filepath.Join(child, s.Name())
			if !isDir(grand) {
				continue
			}
			if info, ok := parseManifest(filepath.Join(grand, "manifest.json")); ok {
				out[info.Type] = append(out[info.Type], info)
				found = true
			}
		}
	}
	return found
}

// parseManifest 解析单个 manifest.json；失败/不合规返回 (PackInfo{}, false)。
func parseManifest(manifestPath string) (PackInfo, bool) {
	if !isFile(manifestPath) {
		return PackInfo{}, false
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return PackInfo{}, false
	}
	var doc manifestDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return PackInfo{}, false
	}
	if doc.Header == nil || doc.Header.UUID == "" || doc.Header.Version == nil {
		return PackInfo{}, false
	}
	// version：仅保留能解析为整数的段，点连接（对齐 Rust filter_map(as_i64)）。
	var parts []string
	for _, n := range doc.Header.Version {
		if i, err := n.Int64(); err == nil {
			parts = append(parts, strconv.FormatInt(i, 10))
		}
	}
	version := strings.Join(parts, ".")

	if doc.Modules == nil {
		return PackInfo{}, false
	}
	var pt PackType
	found := false
	for _, m := range doc.Modules {
		switch m.Type {
		case "data":
			pt = PackBehavior
			found = true
		case "resources":
			pt = PackResource
			found = true
		}
	}
	if !found {
		return PackInfo{}, false
	}
	return PackInfo{Type: pt, UUID: doc.Header.UUID, Version: version, Path: filepath.Dir(manifestPath)}, true
}
