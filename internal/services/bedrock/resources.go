package bedrock

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"io/fs"
)

//go:generate go run genresources.go

// resources.zip 是 ./resources 整树（~26MB → Deflate 5.6MB）的归档，由 genresources.go 生成。
// 为什么不直接 //go:embed all:resources：docs 树里 471+ 个文件名含 CJK 标点（、，（）：《》 等
// 非字母字符），Go embed 的 module.CheckFilePath/fileNameOK 一律拒绝；而相对路径是查询契约不能
// 改名。zip 条目名无此限制，*zip.Reader 直接实现 fs.FS。详见 genresources.go 顶部。
//
//go:embed resources.zip
var resourcesZip []byte

// resourcesFS 是 resources.zip 解出的只读 FS，条目根为 data/ 与 standards/，路径与 cc-pane
// resources 根完全一致（如 "data/api-index.json" / "standards/modsdk-expert.md"），等价于对
// include_dir! 根做 fs.Sub。readResource / walkResourceDir 直接在其上用 fs.ReadFile / fs.WalkDir。
var resourcesFS fs.FS = mustZipFS()

func mustZipFS() fs.FS {
	zr, err := zip.NewReader(bytes.NewReader(resourcesZip), int64(len(resourcesZip)))
	if err != nil {
		// 仅在内嵌 zip 损坏时触发，属打包期错误，直接 panic 暴露。
		panic("bedrock: open embedded resources.zip: " + err.Error())
	}
	return zr
}
