package agentcfg

import (
	"encoding/json"
	"strings"
)

// 本文件是 claude 两层合并模型(系统配置基底 + 供应商凭据补丁)的纯函数工具:
// 深合并、深差异、叶子路径拆分。全部无副作用、不碰 DB/文件,便于单测。

// leafPathSep 是叶子路径各段的分隔符,用 NUL 避免键名本身含分隔符导致的歧义。
const leafPathSep = "\x00"

func cloneValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return cloneMap(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = cloneValue(e)
		}
		return out
	default:
		return t
	}
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneValue(v)
	}
	return out
}

// jsonEqual 用规范化 JSON 文本比较两个值是否语义相等(json.Marshal 对 map 按键排序)。
func jsonEqual(a, b any) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ab) == string(bb)
}

// deepMergeJSON 深合并:patch 覆盖 base。双方同键均为对象 → 递归合并;否则 patch 整体
// 替换(数组不拼接)。仅 base 有 → 保留;仅 patch 有 → 加入。不修改入参,返回全新 map。
func deepMergeJSON(base, patch map[string]any) map[string]any {
	out := cloneMap(base)
	for k, pv := range patch {
		if bv, ok := out[k]; ok {
			if bm, bok := bv.(map[string]any); bok {
				if pm, pok := pv.(map[string]any); pok {
					out[k] = deepMergeJSON(bm, pm)
					continue
				}
			}
		}
		out[k] = cloneValue(pv)
	}
	return out
}

// deepDiffJSON 返回 sub 相对 base 的差异:保留 sub 中「值与 base 不同」或「base 中不存在」
// 的部分。双方同键均为对象 → 递归 diff(子 diff 为空则整键省略);否则按 jsonEqual 比较,
// 相等省略、不等保留 sub 值。满足:当 sub 包含 base 的每个叶子且同值时,
// deepMergeJSON(base, deepDiffJSON(sub, base)) 语义等价于 sub。
func deepDiffJSON(sub, base map[string]any) map[string]any {
	out := map[string]any{}
	for k, sv := range sub {
		bv, inBase := base[k]
		if !inBase {
			out[k] = cloneValue(sv)
			continue
		}
		if sm, sok := sv.(map[string]any); sok {
			if bm, bok := bv.(map[string]any); bok {
				d := deepDiffJSON(sm, bm)
				if len(d) > 0 {
					out[k] = d
				}
				continue
			}
		}
		if !jsonEqual(sv, bv) {
			out[k] = cloneValue(sv)
		}
	}
	return out
}

// flattenLeafPaths 收集 m 的全部叶子路径。叶子 = 非对象值(标量/数组),或空对象。
// 路径段用 leafPathSep 连接。
func flattenLeafPaths(m map[string]any) map[string]struct{} {
	out := map[string]struct{}{}
	var walk func(prefix string, node map[string]any)
	walk = func(prefix string, node map[string]any) {
		if len(node) == 0 {
			if prefix != "" {
				out[prefix] = struct{}{}
			}
			return
		}
		for k, v := range node {
			p := k
			if prefix != "" {
				p = prefix + leafPathSep + k
			}
			if child, ok := v.(map[string]any); ok {
				walk(p, child)
			} else {
				out[p] = struct{}{}
			}
		}
	}
	walk("", m)
	return out
}

func getLeafByPath(root map[string]any, segs []string) any {
	var cur any = root
	for _, s := range segs {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[s]
	}
	return cur
}

// setLeafByPath 在 root 里按 segs 路径建嵌套对象并把末段设为 val。
func setLeafByPath(root map[string]any, segs []string, val any) {
	cur := root
	for i := 0; i < len(segs)-1; i++ {
		next, ok := cur[segs[i]].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[segs[i]] = next
		}
		cur = next
	}
	cur[segs[len(segs)-1]] = val
}

// splitByPaths 把 live 的每个叶子按 pathset 二分:命中 → patchPart,未命中 → basePart,
// 各自按原路径重建嵌套结构。保证 deepMergeJSON(basePart, patchPart) 语义等价于 live。
func splitByPaths(live map[string]any, pathset map[string]struct{}) (patchPart, basePart map[string]any) {
	patchPart = map[string]any{}
	basePart = map[string]any{}
	for p := range flattenLeafPaths(live) {
		segs := strings.Split(p, leafPathSep)
		val := getLeafByPath(live, segs)
		dst := basePart
		if _, hit := pathset[p]; hit {
			dst = patchPart
		}
		setLeafByPath(dst, segs, cloneValue(val))
	}
	return patchPart, basePart
}
