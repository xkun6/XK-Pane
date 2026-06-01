package bedrock

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// userDataRoot 是运行期可选的用户数据根（OTA 同步产物 bedrock-data/ 目录）。空字符串表示
// 仅用内嵌基线。受 rootMu 保护（对齐 mod.rs:39 的进程级 USER_DATA_ROOT RwLock）。
var (
	rootMu       sync.RWMutex
	userDataRoot string
)

// SetDataRoot 安装（或清空）用户数据根。OTA 同步成功后传入新目录；传空字符串回退内嵌基线
// （如 reset 之后）。写入后立即失效全部 loader / index 缓存，下次访问按活跃源重建。
// 对齐 mod.rs:45-49 set_data_root。
func SetDataRoot(root string) {
	rootMu.Lock()
	userDataRoot = root
	rootMu.Unlock()
	invalidateLoaders()
	invalidateIndices()
}

// CurrentDataRoot 返回当前安装的用户数据根（空字符串表示无）。对齐 mod.rs:52-54。
func CurrentDataRoot() string {
	rootMu.RLock()
	defer rootMu.RUnlock()
	return userDataRoot
}

// resourceFile 是 walkResourceDir 的一条结果：相对 relative 的正斜杠路径 + 文件内容。
type resourceFile struct {
	Path    string
	Content []byte
}

// readResource 按相对路径（正斜杠，相对 resources 根）读单个资源文件。对齐 mod.rs:66-79。
//
// 解析序：
//  1. userDataRoot 已设且 <root>/<relative> 存在 → os.ReadFile（读失败报错不回退 ——
//     让调用方看到“同步数据坏了，试试重置”而非掩盖失败）；
//  2. 否则内嵌 resourcesFS。
//
// 两源都无 → err。
func readResource(relative string) ([]byte, error) {
	if root := CurrentDataRoot(); root != "" {
		p := filepath.Join(root, filepath.FromSlash(relative))
		if _, err := os.Stat(p); err == nil {
			b, err := os.ReadFile(p)
			if err != nil {
				return nil, fmt.Errorf("read user data %s: %w", p, err)
			}
			return b, nil
		}
	}
	b, err := fs.ReadFile(resourcesFS, relative)
	if err != nil {
		return nil, fmt.Errorf("resource not found in either source: %s", relative)
	}
	return b, nil
}

// walkResourceDir 遍历 relative 下扩展名匹配 ext（无前导点，大小写不敏感）的全部文件，返回
// (相对 relative 的正斜杠路径, 内容) 列表。对齐 mod.rs:90-167。
//
// 解析：若 <root>/<relative> 是目录 → 整棵替换内嵌子树（**目录级替换，非文件级合并** ——
// 误写成逐文件覆盖会把内嵌全树隐藏）；否则遍历内嵌树。用户侧单文件读失败 continue 容错。
//
// 注：cc-pane 用 std::fs::read_dir（文件系统序，非确定）；Go 的 os.ReadDir / fs.WalkDir 均按
// 文件名字典序，结果更确定（不影响正确性，最终输出都会再排序）。
func walkResourceDir(relative, ext string) []resourceFile {
	if root := CurrentDataRoot(); root != "" {
		userDir := filepath.Join(root, filepath.FromSlash(relative))
		if fi, err := os.Stat(userDir); err == nil && fi.IsDir() {
			var out []resourceFile
			walkUserDir(userDir, userDir, ext, &out)
			return out
		}
	}
	var out []resourceFile
	walkEmbeddedDir(relative, ext, &out)
	return out
}

// walkUserDir 递归遍历用户目录（os 文件系统）。对齐 mod.rs:106-139。
func walkUserDir(root, dir, ext string, out *[]resourceFile) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			walkUserDir(root, p, ext, out)
			continue
		}
		if !matchExt(e.Name(), ext) {
			continue
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		*out = append(*out, resourceFile{Path: filepath.ToSlash(rel), Content: b})
	}
}

// walkEmbeddedDir 递归遍历内嵌子树。对齐 mod.rs:141-167（strip "{base}/" 前缀）。
// base 不存在时安静返回空（对齐 Rust get_dir None）。
func walkEmbeddedDir(base, ext string, out *[]resourceFile) {
	prefix := base + "/"
	_ = fs.WalkDir(resourcesFS, base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // base 缺失等 → 视为空，不传播
		}
		if d.IsDir() {
			return nil
		}
		if !matchExt(p, ext) {
			return nil
		}
		b, rerr := fs.ReadFile(resourcesFS, p)
		if rerr != nil {
			return nil
		}
		*out = append(*out, resourceFile{Path: strings.TrimPrefix(p, prefix), Content: b})
		return nil
	})
}

// matchExt 判定路径/文件名的扩展名（最后一段内、最后一个点之后）是否等于 ext（大小写不敏感、
// 无前导点）。复刻 Rust Path::extension + eq_ignore_ascii_case。无扩展名返回 false。
func matchExt(name, ext string) bool {
	dot := strings.LastIndexByte(name, '.')
	if dot < 0 {
		return false
	}
	if slash := strings.LastIndexAny(name, "/\\"); slash > dot {
		return false // 点在目录段里，文件名本身无扩展
	}
	return strings.EqualFold(name[dot+1:], ext)
}

// fileStem 取相对路径最后一段去扩展名（复刻 Rust Path::file_stem）。
// "modsdk-expert.md"→"modsdk-expert"；"内存优化.md"→"内存优化"；无扩展则返回整段。
func fileStem(rel string) string {
	base := rel
	if slash := strings.LastIndexAny(base, "/\\"); slash >= 0 {
		base = base[slash+1:]
	}
	if dot := strings.LastIndexByte(base, '.'); dot > 0 {
		return base[:dot]
	}
	return base
}
