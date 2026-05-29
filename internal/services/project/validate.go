package project

import (
	"errors"
	"path/filepath"
	"strings"
)

// validatePath 复刻 cc-pane utils/path_validator.rs::validate_path 的三段校验：
// 空 / 含 ".." 父目录分量（路径穿越）/ 非绝对路径。返回的是机器可读的结构化
// 错误码（前端据 code 做 i18n；offending path 由调用方自带，无需随错误回传），
// 区别于 service 层那些纯英文业务错误串。校验顺序与 cc-pane 一致：
// empty → traversal → absolute。
//
// 注意（铁律）：这里只做"拒绝非法输入"，绝不规范化路径——validate_path 通过后
// 路径会被原样存储（不 Clean/Abs/EvalSymlinks/小写化/去尾分隔符/去 \\?\ 前缀）。
var (
	ErrPathEmpty       = errors.New("PATH_EMPTY")
	ErrPathTraversal   = errors.New("PATH_TRAVERSAL")
	ErrPathNotAbsolute = errors.New("PATH_NOT_ABSOLUTE")
)

func validatePath(path string) error {
	if path == "" {
		return ErrPathEmpty
	}
	// 任一以 / 或 \ 分隔的分量等于 ".." 即判路径穿越（等价 Rust
	// Component::ParentDir，同时兼容两种分隔符，跨平台一致）。
	for _, seg := range strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if seg == ".." {
			return ErrPathTraversal
		}
	}
	if !filepath.IsAbs(path) {
		return ErrPathNotAbsolute
	}
	return nil
}

// deriveName 复刻 Project::new 的命名：取路径最后一个分量（file_name）；无有效
// 分量时回退字面 "unnamed"（对齐 PathBuf::file_name().and_then(to_str) 的 None）。
// filepath.Base 已会剥掉尾部分隔符（"C:\foo\" → "foo"），但根类路径会回出纯
// 分隔符（"C:\" → "\"）或 "."，这些归入 unnamed 回退。
func deriveName(path string) string {
	base := filepath.Base(path)
	switch base {
	case "", ".", "/", "\\": // 两种分隔符字面覆盖任意平台的根类回出值
		return "unnamed"
	}
	return base
}
