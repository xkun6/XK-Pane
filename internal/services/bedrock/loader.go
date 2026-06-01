package bedrock

import (
	"encoding/json"
	"fmt"
	"sync"
	"unicode/utf8"
)

// 四槽惰性加载缓存：apis / components / templates / standards。对齐 loader.rs。
//
// cc-pane 用 RwLock<Option<&'static T>> + Box::leak 持有 &'static（invalidate 后旧 leak 无法
// 回收）。Go 改普通指针 + GC 替代 Box::leak（明确简化点）：每槽 RWMutex + 指针/map，nil 判定
// 重建。因本轮做 OTA 需 invalidate，**不用 sync.Once**（不可重置）。
var (
	apisMu          sync.RWMutex
	apisCache       *[]ApiEntry
	componentsMu    sync.RWMutex
	componentsCache *[]ComponentEntry
	templatesMu     sync.RWMutex
	templatesCache  *[]TemplateEntry
	standardsMu     sync.RWMutex
	standardsCache  map[string]string
)

// invalidateLoaders 丢弃全部 loader 缓存，下次访问按当前安装的数据源重建。对齐 loader.rs:40-45。
func invalidateLoaders() {
	apisMu.Lock()
	apisCache = nil
	apisMu.Unlock()
	componentsMu.Lock()
	componentsCache = nil
	componentsMu.Unlock()
	templatesMu.Lock()
	templatesCache = nil
	templatesMu.Unlock()
	standardsMu.Lock()
	standardsCache = nil
	standardsMu.Unlock()
}

// loadApis 加载并缓存 data/api-index.json → []ApiEntry（对齐 loader.rs:56-66）。
func loadApis() ([]ApiEntry, error) {
	apisMu.RLock()
	if apisCache != nil {
		v := *apisCache
		apisMu.RUnlock()
		return v, nil
	}
	apisMu.RUnlock()

	b, err := readResource("data/api-index.json")
	if err != nil {
		return nil, fmt.Errorf("read data/api-index.json: %w", err)
	}
	var parsed []ApiEntry
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, fmt.Errorf("parse api-index.json: %w", err)
	}
	apisMu.Lock()
	apisCache = &parsed
	apisMu.Unlock()
	return parsed, nil
}

// loadComponents 加载并缓存 data/components-index.json → []ComponentEntry（对齐 loader.rs:69-80）。
func loadComponents() ([]ComponentEntry, error) {
	componentsMu.RLock()
	if componentsCache != nil {
		v := *componentsCache
		componentsMu.RUnlock()
		return v, nil
	}
	componentsMu.RUnlock()

	b, err := readResource("data/components-index.json")
	if err != nil {
		return nil, fmt.Errorf("read data/components-index.json: %w", err)
	}
	var parsed []ComponentEntry
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, fmt.Errorf("parse components-index.json: %w", err)
	}
	componentsMu.Lock()
	componentsCache = &parsed
	componentsMu.Unlock()
	return parsed, nil
}

// loadTemplates 加载并缓存 data/templates-index.json → []TemplateEntry（对齐 loader.rs:83-94）。
func loadTemplates() ([]TemplateEntry, error) {
	templatesMu.RLock()
	if templatesCache != nil {
		v := *templatesCache
		templatesMu.RUnlock()
		return v, nil
	}
	templatesMu.RUnlock()

	b, err := readResource("data/templates-index.json")
	if err != nil {
		return nil, fmt.Errorf("read data/templates-index.json: %w", err)
	}
	var parsed []TemplateEntry
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, fmt.Errorf("parse templates-index.json: %w", err)
	}
	templatesMu.Lock()
	templatesCache = &parsed
	templatesMu.Unlock()
	return parsed, nil
}

// loadStandards 加载并缓存全部 standards/*.md，key = 文件 stem（去 .md，含 CJK 名如「内存优化」）。
// 非 UTF-8 内容跳过（对齐 loader.rs:99-123）。
func loadStandards() (map[string]string, error) {
	standardsMu.RLock()
	if standardsCache != nil {
		v := standardsCache
		standardsMu.RUnlock()
		return v, nil
	}
	standardsMu.RUnlock()

	files := walkResourceDir("standards", "md")
	m := make(map[string]string, len(files))
	for _, f := range files {
		stem := fileStem(f.Path)
		if stem == "" {
			continue
		}
		if !utf8.Valid(f.Content) {
			continue
		}
		m[stem] = string(f.Content)
	}
	standardsMu.Lock()
	standardsCache = m
	standardsMu.Unlock()
	return m, nil
}
