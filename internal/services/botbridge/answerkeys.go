package botbridge

import "sort"

const (
	keyDown  = "\x1b[B"
	keyRight = "\x1b[C"
	keyEnter = "\r"
	keySpace = " "
)

type AnswerOption struct{}

type AnswerQuestion struct {
	MultiSelect bool
	Options     []AnswerOption
}

// BuildAnswerKeys 对齐前端 SessionTimeline/answerKeys.ts：返回逐键写入 PTY 的按键序列。
func BuildAnswerKeys(questions []AnswerQuestion, selections map[int][]int) []string {
	n := len(questions)
	hideSubmitTab := n == 1 && !questions[0].MultiSelect
	keys := []string{}

	for i, q := range questions {
		count := len(q.Options)
		sel := filterSelection(selections[i], count)
		if q.MultiSelect {
			targets := dedupSorted(sel)
			pos := 0
			for _, t := range targets {
				for d := 0; d < t-pos; d++ {
					keys = append(keys, keyDown)
				}
				keys = append(keys, keySpace)
				pos = t
			}
			keys = append(keys, keyRight)
			continue
		}
		t := 0
		if len(sel) > 0 {
			t = sel[0]
		}
		for d := 0; d < t; d++ {
			keys = append(keys, keyDown)
		}
		keys = append(keys, keyEnter)
	}
	if !hideSubmitTab {
		keys = append(keys, keyEnter)
	}
	return keys
}

func filterSelection(in []int, count int) []int {
	out := make([]int, 0, len(in))
	for _, x := range in {
		if x >= 0 && x < count {
			out = append(out, x)
		}
	}
	return out
}

func dedupSorted(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	sort.Ints(in)
	out := in[:0]
	last := -1
	for _, x := range in {
		if len(out) == 0 || x != last {
			out = append(out, x)
			last = x
		}
	}
	return out
}
