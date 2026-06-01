package bedrock

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validate_assets.go：bedrock_validate_addon_assets。扫 addon 根下 textures/ 的图片，逐图 A1-A4 检查。
// 对齐 validate/assets.rs。

var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

const sizeWarnBytes uint64 = 1024 * 1024

type imageKind int

const (
	kindPng imageKind = iota
	kindTga
	kindOtherImage
	kindNotImage
)

// classifyImage：扩展名大小写不敏感（**与 json_defs 大小写敏感不同**）。对齐 assets.rs:36-43。
func classifyImage(ext string) imageKind {
	switch strings.ToLower(ext) {
	case "png":
		return kindPng
	case "tga":
		return kindTga
	case "jpg", "jpeg", "bmp", "gif", "webp":
		return kindOtherImage
	default:
		return kindNotImage
	}
}

// validateAddonAssets ← bedrock_validate_addon_assets。
func validateAddonAssets(addonDir string) (string, error) {
	if msg, bad := checkAddonPath(addonDir, "Addon Asset Validation"); bad {
		return msg, nil
	}

	title := fmt.Sprintf("Addon Asset Validation: %s", addonDir)
	textures := filepath.Join(addonDir, "textures")
	var issues []issue
	scanned := 0

	if fi, err := os.Stat(textures); err != nil || !fi.IsDir() {
		issues = append(issues, issue{issueInfo, "(scan)", "no textures/ directory at addon root; nothing to check"})
		return renderValidateReport(title, scanned, issues), nil
	}

	var files []string
	collectAllFiles(textures, &files)

	for _, f := range files {
		ext := extOf(filepath.Base(f))
		kind := classifyImage(ext)
		if kind == kindNotImage {
			continue // 非图片文件静默跳过、不计数
		}
		scanned++
		checkImage(addonDir, f, kind, ext, &issues)
	}
	return renderValidateReport(title, scanned, issues), nil
}

// collectAllFiles 递归收集全部普通文件（任意扩展名）。对齐 assets.rs:104-116。
func collectAllFiles(dir string, out *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			collectAllFiles(p, out)
		} else if e.Type().IsRegular() {
			*out = append(*out, p)
		}
	}
}

func checkImage(root, file string, kind imageKind, ext string, issues *[]issue) {
	rel := relativePath(root, file)

	// A1 — format
	if kind == kindOtherImage {
		*issues = append(*issues, issue{issueWarning, rel, fmt.Sprintf("unsupported image format: .%s; prefer PNG", ext)})
	}

	// A3 — filename
	name := filepath.Base(file)
	if strings.Contains(name, " ") {
		*issues = append(*issues, issue{issueWarning, rel, "filename contains spaces; rename without whitespace"})
	}
	if hasASCIIUpper(name) {
		*issues = append(*issues, issue{issueWarning, rel, "filename has uppercase characters; use all lowercase"})
	}
	if hasBadFilenameChar(name) {
		*issues = append(*issues, issue{issueInfo, rel, "filename has characters outside [a-z0-9_.-]; sanitise"})
	}

	// A4 — size
	var size uint64
	if fi, err := os.Stat(file); err == nil {
		size = uint64(fi.Size())
	}
	if size > sizeWarnBytes {
		kb := (size + 1023) / 1024 // div_ceil(1024)
		*issues = append(*issues, issue{issueWarning, rel, fmt.Sprintf("large texture: %d KB; consider downscaling or PNG-8", kb)})
	}

	// A2 — PNG dimensions
	if kind == kindPng {
		w, h, ok, err := readPNGDims(file)
		switch {
		case err != nil:
			*issues = append(*issues, issue{issueWarning, rel, fmt.Sprintf("could not inspect PNG header: %v", err)})
		case !ok:
			*issues = append(*issues, issue{issueWarning, rel, "PNG header missing or corrupt; could not read dimensions"})
		default:
			if !isPowerOfTwo(w) || !isPowerOfTwo(h) {
				*issues = append(*issues, issue{issueWarning, rel, fmt.Sprintf("dimensions %dx%d are not power-of-two", w, h)})
			}
			if !aspectOK(w, h) {
				*issues = append(*issues, issue{issueInfo, rel, fmt.Sprintf("aspect ratio %d:%d is unusual; prefer 1:1, 1:2, or 2:1", w, h)})
			}
		}
	}
}

func hasASCIIUpper(name string) bool {
	for i := 0; i < len(name); i++ {
		if name[i] >= 'A' && name[i] <= 'Z' {
			return true
		}
	}
	return false
}

func hasBadFilenameChar(name string) bool {
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || isASCIIDigit(c) || c == '_' || c == '.' || c == '-') {
			return true
		}
	}
	return false
}

func isPowerOfTwo(n uint32) bool { return n > 0 && (n&(n-1)) == 0 }

func aspectOK(w, h uint32) bool {
	if w == 0 || h == 0 {
		return false
	}
	return w == h || w == h*2 || h == w*2
}

// readPNGDims 手解析 PNG IHDR 取宽高（BigEndian，offset 16-23）。ok=false&err=nil → 非 PNG/过短；
// err!=nil → 读失败。对齐 assets.rs:212-230。
func readPNGDims(path string) (uint32, uint32, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, false, err
	}
	if len(b) < 24 {
		return 0, 0, false, nil
	}
	if !bytes.Equal(b[:8], pngMagic) {
		return 0, 0, false, nil
	}
	if string(b[12:16]) != "IHDR" {
		return 0, 0, false, nil
	}
	w := binary.BigEndian.Uint32(b[16:20])
	h := binary.BigEndian.Uint32(b[20:24])
	return w, h, true, nil
}
