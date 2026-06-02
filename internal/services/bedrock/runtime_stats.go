package bedrock

// runtime_stats.go：暴露「当前 loader 可服务的轻量计数快照」给 OTA 同步子系统（bedrockdocsync），
// 让它在原子换目录 + SetDataRoot 之后据此填 manifest.stats。返回独立 struct（非 bedrockdocsync 的
// 类型）以避免 bedrock ↔ bedrockdocsync 循环 import —— 由 app 装配处做一次字段转换。
// 对齐 cc-pane service.rs:compute_runtime_stats。

// Stats 是运行期可服务（用户数据 ∪ 内嵌基线）的头条计数快照。
type Stats struct {
	APIEntries      uint32
	TemplateEntries uint32
	GuideFiles      uint32
}

// guideStatSubdirs 是计入 guide 文件数的 data/docs 子目录（对齐 compute_runtime_stats 的五源）。
// 任何未来新增子目录会被静默漏掉（非阻塞）。
var guideStatSubdirs = []string{"mcguide", "mconline", "mcdocs", "wiki", "ui_templates"}

// RuntimeStats 直接问 loader 算出当前可查询的 API / 模板 / 指南 markdown 数（双源合并计数，与
// OTA 状态无关）。调用前若刚原子换目录，应先 SetDataRoot 失效缓存，否则仍读旧源（对齐
// service.rs 在 set_data_root 之后才 compute_runtime_stats 的顺序）。
func RuntimeStats() Stats {
	var apiCount, tmplCount uint32
	if apis, err := loadApis(); err == nil {
		apiCount = uint32(len(apis))
	}
	if tmpls, err := loadTemplates(); err == nil {
		tmplCount = uint32(len(tmpls))
	}
	var guideFiles uint32
	for _, sub := range guideStatSubdirs {
		guideFiles += uint32(len(walkResourceDir("data/docs/"+sub, "md")))
	}
	return Stats{APIEntries: apiCount, TemplateEntries: tmplCount, GuideFiles: guideFiles}
}
