//go:build ignore

// genresources 把 ./resources 整树打包成 ./resources.zip 供 resources.go 经 //go:embed 内嵌。
//
// 为什么要 zip 而不是直接 //go:embed all:resources：Go 的 embed 用 module.CheckFilePath →
// fileNameOK 校验每个文件名，非 ASCII 字符**只允许 unicode.IsLetter**。docs 树里 471+ 个文件
// 名含 CJK 标点（、，（）：《》 等，均非字母），go:embed 一律拒绝。而文件相对路径是查询契约
// （GuideEntry.Path / API.Link / bedrock_read_guide 的 path 参数都依赖原始路径），不能改名。
// zip 归档对条目名无此限制，且 *zip.Reader 直接实现 fs.FS，运行期零额外解析成本。
//
// 用法（更新基线数据后重新生成，需 ./resources 树在位 —— 通常从 cc-pane 拷贝）：
//
//	cd internal/services/bedrock && go run genresources.go
//
// 生成的 resources.zip 已 commit（.gitattributes 标记 -text 防 CRLF）；./resources 树本身
// 不入库（.gitignore），仅作为重生成源。Deflate 压缩、固定时间戳、字典序条目 → 可复现。
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	srcDir = "resources"
	outZip = "resources.zip"
)

// fixedModTime 固定条目时间戳，使重复生成的 zip 字节可复现（内容字节始终保真）。
var fixedModTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genresources:", err)
		os.Exit(1)
	}
}

func run() error {
	out, err := os.Create(outZip)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	count := 0

	// WalkDir 字典序遍历 → 条目顺序确定。条目名相对 srcDir，正斜杠（如 data/api-index.json）。
	err = filepath.WalkDir(srcDir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)

		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
		hdr.Modified = fixedModTime
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		f.Close()
		if err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		zw.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	fmt.Printf("genresources: 写入 %s（%d 个文件）\n", outZip, count)
	return nil
}
