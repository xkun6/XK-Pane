package localhistory

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// diff 引擎：移植 cc-pane history_file_repo.rs compute_diff（行级 hunk + 字符级 inline）。
//
// 决策门 D11（用户授权我选库）：用 `github.com/sergi/go-diff/diffmatchpatch`——它**已是间接
// 依赖**（go-git/wails 引入），promote 到 direct **零新依赖**，且单库即可做行级（DiffLinesToChars
// 行模式）+ 字符级（DiffMain 字符模式）；优于另引 go-udiff。行为对齐 cc-pane 的 similar：
// 行级 Equal/Delete/Insert → 按 CONTEXT_LINES=3 取上下文、相邻变更段（间距 ≤ 2*CONTEXT）合并成
// hunk；相邻 Delete/Insert 行配对做字符级 inline；字符位置用 **rune 计数**（非字节，对齐 JS 前端）。

const (
	contextLines = 3
	maxDiffLines = 10000
)

// DiffChangeType 变更类型（JSON snake_case，对齐 cc-pane）。
type DiffChangeType string

const (
	ChangeEqual   DiffChangeType = "equal"
	ChangeInsert  DiffChangeType = "insert"
	ChangeDelete  DiffChangeType = "delete"
	ChangeReplace DiffChangeType = "replace"
)

// InlineChange 行内字符级变更标记（start/end 为 rune 下标）。
type InlineChange struct {
	Start      int            `json:"start"`
	End        int            `json:"end"`
	ChangeType DiffChangeType `json:"changeType"`
}

// DiffLine diff 中的单行（OldLineNo/NewLineNo 为 nil 表示该侧无此行）。
type DiffLine struct {
	ChangeType    DiffChangeType `json:"changeType"`
	Content       string         `json:"content"`
	OldLineNo     *int           `json:"oldLineNo"`
	NewLineNo     *int           `json:"newLineNo"`
	InlineChanges []InlineChange `json:"inlineChanges,omitempty"`
}

// DiffStats diff 统计。
type DiffStats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Changes   int `json:"changes"`
}

// DiffHunk 一段连续变更（含上下文）。
type DiffHunk struct {
	OldStart int        `json:"oldStart"`
	OldCount int        `json:"oldCount"`
	NewStart int        `json:"newStart"`
	NewCount int        `json:"newCount"`
	Lines    []DiffLine `json:"lines"`
}

// DiffResult diff 结果。
type DiffResult struct {
	Hunks     []DiffHunk `json:"hunks"`
	Stats     DiffStats  `json:"stats"`
	IsBinary  bool       `json:"isBinary"`
	Truncated bool       `json:"truncated"`
}

// IsBinary 是 isBinary 的导出包装，供 lhwatcher 在保存前做二进制过滤（单一事实源，对齐 diff 引擎
// 的判定，避免 watcher 自带一份易漂移的副本）。
func IsBinary(content []byte) bool { return isBinary(content) }

// isBinary 检测内容是否二进制：先看 BOM（有 BOM 即文本），再看首 8KB 是否含 \0。对齐 cc-pane。
func isBinary(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	switch {
	case len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF: // UTF-8 BOM
		return false
	case len(content) >= 2 && content[0] == 0xFF && content[1] == 0xFE: // UTF-16/32 LE BOM
		return false
	case len(content) >= 2 && content[0] == 0xFE && content[1] == 0xFF: // UTF-16 BE BOM
		return false
	case len(content) >= 4 && content[0] == 0x00 && content[1] == 0x00 && content[2] == 0xFE && content[3] == 0xFF: // UTF-32 BE BOM
		return false
	}
	checkLen := len(content)
	if checkLen > 8192 {
		checkLen = 8192
	}
	return bytes.IndexByte(content[:checkLen], 0) >= 0
}

// splitLines 仅用于 MAX_DIFF_LINES 的**行数统计**（对齐 cc-pane 用 str::lines() 计数做
// truncated 判断），不参与每行 content 的生成（content 走 splitDiffBlock）。以 \n 分隔、文本
// 以 \n 结尾时不产生末尾空行；去尾 \r 仅为稳健（ComputeDiff 已先归一化，通常无 \r）。
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1] // 去掉末尾 \n 造成的空段
	}
	for i := range parts {
		parts[i] = strings.TrimSuffix(parts[i], "\r")
	}
	return parts
}

type lineChange struct {
	tag     diffmatchpatch.Operation
	oldNo   int
	newNo   int
	content string
}

// splitDiffBlock 把一个 diff 块的文本（若干整行的拼接，每行含 \n，末行可能无）拆成各行内容
// （去尾 \n）。输入已在 ComputeDiff 经 normalizeNewlines 归一为仅 \n，故无需再处理 \r。
func splitDiffBlock(text string) []string {
	if text == "" {
		return nil
	}
	segs := strings.SplitAfter(text, "\n")
	out := make([]string, 0, len(segs))
	for _, seg := range segs {
		if seg == "" {
			continue // 末尾 \n 造成的空段
		}
		out = append(out, strings.TrimSuffix(seg, "\n"))
	}
	return out
}

// normalizeNewlines 把 \r\n 与孤立 \r 统一为 \n（用于 diff 计算，见 ComputeDiff 注释）。
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// lineChanges 行级 diff：用 diffmatchpatch 行模式（DiffLinesToChars）得到逐行 Equal/Delete/Insert，
// 并赋老/新行号（对齐 cc-pane similar from_lines + iter_all_changes）。
func lineChanges(oldText, newText string) []lineChange {
	dmp := diffmatchpatch.New()
	a, b, lineArray := dmp.DiffLinesToChars(oldText, newText)
	diffs := dmp.DiffCharsToLines(dmp.DiffMain(a, b, false), lineArray)

	out := make([]lineChange, 0)
	oldNo, newNo := 0, 0
	for _, d := range diffs {
		for _, content := range splitDiffBlock(d.Text) {
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				oldNo++
				newNo++
				out = append(out, lineChange{diffmatchpatch.DiffEqual, oldNo, newNo, content})
			case diffmatchpatch.DiffDelete:
				oldNo++
				out = append(out, lineChange{diffmatchpatch.DiffDelete, oldNo, 0, content})
			case diffmatchpatch.DiffInsert:
				newNo++
				out = append(out, lineChange{diffmatchpatch.DiffInsert, 0, newNo, content})
			}
		}
	}
	return out
}

func iptr(n int) *int { return &n }

// ComputeDiff 计算两段文本的行级 + 字符级 diff。二进制 / 超 MAX_DIFF_LINES 时返回相应标志位。
func ComputeDiff(oldText, newText string) DiffResult {
	if isBinary([]byte(oldText)) || isBinary([]byte(newText)) {
		return DiffResult{Hunks: []DiffHunk{}, IsBinary: true}
	}
	// 换行归一化（仅作用于 diff 计算/显示，绝不影响存储 blob 与 restore 的原始字节）。
	// **有意改进 cc-pane**：cc-pane（similar from_lines）换行敏感——CRLF↔LF 会把整文件判为
	// 全行替换（假阳性「全爆红」），而 GetVersionDiff 拿存储版本(可能 CRLF)对比工作树文件
	// (可能被编辑器/Git 规范化为 LF)极易触发。这里先把 \r\n / 孤立 \r 归一为 \n，使纯行尾
	// 风格差异不再产生伪 hunk，真实内容变更照常呈现。（对抗审查 workflow w05btokta 确认的修复）
	oldText = normalizeNewlines(oldText)
	newText = normalizeNewlines(newText)

	if len(splitLines(oldText)) > maxDiffLines || len(splitLines(newText)) > maxDiffLines {
		return DiffResult{Hunks: []DiffHunk{}, Truncated: true}
	}

	all := lineChanges(oldText, newText)

	// 找出所有连续的非 Equal 段。
	var ranges [][2]int
	for i := 0; i < len(all); {
		if all[i].tag != diffmatchpatch.DiffEqual {
			start := i
			for i < len(all) && all[i].tag != diffmatchpatch.DiffEqual {
				i++
			}
			ranges = append(ranges, [2]int{start, i})
		} else {
			i++
		}
	}
	// 合并间距 ≤ 2*contextLines 的相邻段。
	var merged [][2]int
	for _, r := range ranges {
		if n := len(merged); n > 0 && r[0] <= merged[n-1][1]+2*contextLines {
			merged[n-1][1] = r[1]
		} else {
			merged = append(merged, r)
		}
	}

	hunks := make([]DiffHunk, 0, len(merged))
	stats := DiffStats{}
	for _, m := range merged {
		hunkStart := m[0] - contextLines
		if hunkStart < 0 {
			hunkStart = 0
		}
		hunkEnd := m[1] + contextLines
		if hunkEnd > len(all) {
			hunkEnd = len(all)
		}
		lines := make([]DiffLine, 0, hunkEnd-hunkStart)
		var hOldStart, hNewStart, hOldCount, hNewCount int
		for idx := hunkStart; idx < hunkEnd; idx++ {
			c := all[idx]
			if idx == hunkStart {
				if hOldStart = c.oldNo; hOldStart == 0 {
					hOldStart = 1
				}
				if hNewStart = c.newNo; hNewStart == 0 {
					hNewStart = 1
				}
			}
			switch c.tag {
			case diffmatchpatch.DiffEqual:
				lines = append(lines, DiffLine{ChangeType: ChangeEqual, Content: c.content, OldLineNo: iptr(c.oldNo), NewLineNo: iptr(c.newNo)})
				hOldCount++
				hNewCount++
			case diffmatchpatch.DiffDelete:
				stats.Deletions++
				lines = append(lines, DiffLine{ChangeType: ChangeDelete, Content: c.content, OldLineNo: iptr(c.oldNo)})
				hOldCount++
			case diffmatchpatch.DiffInsert:
				stats.Additions++
				lines = append(lines, DiffLine{ChangeType: ChangeInsert, Content: c.content, NewLineNo: iptr(c.newNo)})
				hNewCount++
			}
		}
		computeInlineChanges(lines)
		hunks = append(hunks, DiffHunk{OldStart: hOldStart, OldCount: hOldCount, NewStart: hNewStart, NewCount: hNewCount, Lines: lines})
	}
	stats.Changes = min(stats.Additions, stats.Deletions)

	return DiffResult{Hunks: hunks, Stats: stats}
}

// computeInlineChanges 对相邻的 Delete 段与紧随的 Insert 段一一配对，做字符级 inline diff。
// 字符位置用 rune 计数（与前端 JS String 一致）。对齐 cc-pane compute_inline_changes。
func computeInlineChanges(lines []DiffLine) {
	dmp := diffmatchpatch.New()
	for i := 0; i < len(lines); {
		delStart := i
		for i < len(lines) && lines[i].ChangeType == ChangeDelete {
			i++
		}
		delEnd := i
		insStart := i
		for i < len(lines) && lines[i].ChangeType == ChangeInsert {
			i++
		}
		insEnd := i

		delCount := delEnd - delStart
		insCount := insEnd - insStart
		pairs := min(delCount, insCount)
		for p := 0; p < pairs; p++ {
			oldStr := lines[delStart+p].Content
			newStr := lines[insStart+p].Content
			var oldInlines, newInlines []InlineChange
			oldPos, newPos := 0, 0
			for _, cd := range dmp.DiffMain(oldStr, newStr, false) {
				n := utf8.RuneCountInString(cd.Text)
				switch cd.Type {
				case diffmatchpatch.DiffEqual:
					oldPos += n
					newPos += n
				case diffmatchpatch.DiffDelete:
					oldInlines = append(oldInlines, InlineChange{Start: oldPos, End: oldPos + n, ChangeType: ChangeDelete})
					oldPos += n
				case diffmatchpatch.DiffInsert:
					newInlines = append(newInlines, InlineChange{Start: newPos, End: newPos + n, ChangeType: ChangeInsert})
					newPos += n
				}
			}
			if len(oldInlines) > 0 {
				lines[delStart+p].InlineChanges = oldInlines
			}
			if len(newInlines) > 0 {
				lines[insStart+p].InlineChanges = newInlines
			}
		}
		if delCount == 0 && insCount == 0 {
			i++ // 当前是 Equal 行，前移避免死循环
		}
	}
}

// GetVersionDiff 返回某版本与**当前工作树文件**的 diff（工作树文件不存在则视为空，等价删除全部）。
func (s *Store) GetVersionDiff(ctx context.Context, filePath, versionID string) (DiffResult, error) {
	oldContent, err := s.GetVersionContent(ctx, filePath, versionID)
	if err != nil {
		return DiffResult{}, err
	}
	full := filepath.Join(s.projectPath, filepath.FromSlash(filePath))
	newBytes, _ := os.ReadFile(full) // 读失败（已删）→ 空内容
	return ComputeDiff(string(oldContent), string(newBytes)), nil
}

// GetVersionsDiff 返回两个版本之间的 diff。
func (s *Store) GetVersionsDiff(ctx context.Context, filePath, oldVersionID, newVersionID string) (DiffResult, error) {
	oldContent, err := s.GetVersionContent(ctx, filePath, oldVersionID)
	if err != nil {
		return DiffResult{}, err
	}
	newContent, err := s.GetVersionContent(ctx, filePath, newVersionID)
	if err != nil {
		return DiffResult{}, err
	}
	return ComputeDiff(string(oldContent), string(newContent)), nil
}
