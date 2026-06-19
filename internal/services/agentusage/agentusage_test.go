package agentusage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newAt 构造一个 homeDir 指向 tmp 的 service（绕过 os.UserHomeDir，便于喂测试数据）。
func newAt(home string) *Service {
	return &Service{files: make(map[string]*fileState), homeDir: home}
}

// asstLine 拼一条带 usage 的 assistant 转录行（含去重用的 message.id 与 model）。
func asstLine(id, model string, ts time.Time, in, out, cc, cr int) string {
	return fmt.Sprintf(
		`{"type":"assistant","timestamp":"%s","message":{"id":"%s","model":"%s","usage":{"input_tokens":%d,"output_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d}}}`,
		ts.Format(time.RFC3339), id, model, in, out, cc, cr,
	)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func TestCollect(t *testing.T) {
	home := t.TempDir()
	sess := filepath.Join(home, ".claude", "projects", "projA", "a.jsonl")
	now := time.Now()

	// e1/e2 在 5h 窗口内；e3 时间戳 10 天前（文件 mtime 新→会被扫，但事件落在周窗外）；
	// dup 复用 e2 的 id（连续重复行应被去重）；synthetic 行 model=<synthetic> 应被剔除。
	lines := []string{
		asstLine("m1", "claude-opus", now.Add(-1*time.Hour), 100, 10, 5, 20), // tot 135
		asstLine("m2", "claude-opus", now.Add(-3*time.Hour), 200, 20, 0, 0),   // tot 220
		asstLine("m2", "claude-opus", now.Add(-3*time.Hour), 200, 20, 0, 0),   // dup → 忽略
		asstLine("syn", "<synthetic>", now.Add(-1*time.Hour), 999, 0, 0, 0),   // synthetic → 忽略
		asstLine("m3", "claude-opus", now.Add(-10*24*time.Hour), 1000, 0, 0, 0), // 周窗外
		`{"type":"user","message":{"role":"user"}}`,                            // 非 assistant → 预筛跳过
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	writeFile(t, sess, content)

	s := newAt(home)
	u := s.collect()

	if !u.Available {
		t.Fatal("Available 应为 true（projects 目录存在）")
	}
	// e1+e2 落在 5h 与周窗；e3 仅在文件层被扫到但落在周窗外；dup/synthetic/user 全部排除。
	if u.Last5h.Total != 355 || u.Last5h.Msgs != 2 {
		t.Errorf("Last5h = %+v，期望 Total=355 Msgs=2", u.Last5h)
	}
	if u.Week.Total != 355 || u.Week.Msgs != 2 {
		t.Errorf("Week = %+v，期望 Total=355 Msgs=2（10 天前事件应排除）", u.Week)
	}
	// 明细拆分核对（e1+e2）：input 300 / output 30 / cacheCreate 5 / cacheRead 20。
	if u.Last5h.Input != 300 || u.Last5h.Output != 30 || u.Last5h.CacheCreate != 5 || u.Last5h.CacheRead != 20 {
		t.Errorf("Last5h 明细 = %+v，期望 in300 out30 cc5 cr20", u.Last5h)
	}

	// —— 增量：追加 e4（完整行）后再聚合，应只多算 e4、不重复既有 ——
	appendFile(t, sess, asstLine("m4", "claude-opus", now.Add(-30*time.Minute), 50, 0, 0, 0)+"\n")
	u2 := s.collect()
	if u2.Last5h.Total != 405 || u2.Last5h.Msgs != 3 {
		t.Errorf("增量后 Last5h = %+v，期望 Total=405 Msgs=3", u2.Last5h)
	}

	// —— 半行：写入无结尾换行的一行，本轮不应消费（offset 停在上一个完整换行）——
	half := asstLine("m5", "claude-opus", now.Add(-10*time.Minute), 7, 0, 0, 0) // 不带 \n
	appendFile(t, sess, half)
	u3 := s.collect()
	if u3.Last5h.Total != 405 {
		t.Errorf("半行不应被消费，Last5h.Total = %d，期望仍为 405", u3.Last5h.Total)
	}
	// 补上换行后应被消费。
	appendFile(t, sess, "\n")
	u4 := s.collect()
	if u4.Last5h.Total != 412 || u4.Last5h.Msgs != 4 {
		t.Errorf("补换行后 Last5h = %+v，期望 Total=412 Msgs=4", u4.Last5h)
	}
}

// TestMonthWindow 校验「本月」按自然月分桶（月初 00:00 起），不受 5h/周等滑动窗口影响。
// 用 monthStart±1h 构造跨月边界内外两条事件，断言只有月内那条进 Month —— 不依赖运行时具体日期。
func TestMonthWindow(t *testing.T) {
	home := t.TempDir()
	sess := filepath.Join(home, ".claude", "projects", "projM", "m.jsonl")
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	in := asstLine("in", "claude-opus", monthStart.Add(1*time.Hour), 500, 0, 0, 0)   // 月内 → 计入
	out := asstLine("out", "claude-opus", monthStart.Add(-1*time.Hour), 700, 0, 0, 0) // 上月末 → 不计
	writeFile(t, sess, in+"\n"+out+"\n")

	u := newAt(home).collect()
	if u.Month.Total != 500 || u.Month.Msgs != 1 {
		t.Errorf("Month = %+v，期望 Total=500 Msgs=1（仅月内事件）", u.Month)
	}
}

func TestCollectNoClaudeDir(t *testing.T) {
	s := newAt(t.TempDir()) // 没有 .claude/projects
	u := s.collect()
	if u.Available {
		t.Error("无 ~/.claude/projects 时 Available 应为 false")
	}
	if u.Week.Total != 0 {
		t.Errorf("无数据时 Week.Total 应为 0，得 %d", u.Week.Total)
	}
}
