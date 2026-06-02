package bedrockdocsync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// build_templates_index.go：扫描 Python 示例项目树生成 templates-index.json（对齐
// build_templates_index.rs）。每个 demo 子目录 = 一个 mod。纯 std 风格：无 regexp、无 walkdir，
// 用按字节状态机识别 ListenForEvent("Event") / comp.Method( / compFactory.CreateXxx( /
// RegisterSystem("ns","name","cls") / @Mod.Binding(name="...") / ModName="..." 等锚点。
//
// 注：文件头注释提到检测 `class XxxSystem(ServerSystem|ClientSystem)`，但 Rust 代码并未实现
// system-class 扫描 —— systems 全部来自 RegisterSystem(...) 三参解析（以代码为准）。
//
// 输出 JSON tag 必须 snake_case（被 bedrock loader 的 TemplateEntry 回读），struct 字段顺序严格
// 对齐 serde Serialize（按字段声明序）。systems/apis/file_count 的内部 key 序对齐 serde_json
// 默认 BTreeMap 字典序（Cargo.toml 无 preserve_order）：systems 对象 → cls_path/name/namespace，
// apis → components/methods，file_count → 分类名字典序。

// OutputTemplateEntry 是输出条目形状（对齐 OutputTemplateEntry，序列化专用）。字段声明顺序即
// serde 序列化顺序，勿调整。
type OutputTemplateEntry struct {
	ID              string            `json:"id"`
	Path            string            `json:"path"`
	ModName         string            `json:"mod_name"`
	BehaviorPack    string            `json:"behavior_pack"`
	ResourcePack    string            `json:"resource_pack"`
	Tags            []string          `json:"tags"`        // 非 nil；空时回退 [project_name]
	Systems         []systemEntry     `json:"systems"`     // 非 nil；保遍历序，不排序不去重
	EventsUsed      []string          `json:"events_used"` // 非 nil；字典序去重
	APIs            apisValue         `json:"apis"`        // {components,methods}，各自字典序去重
	HasClient       bool              `json:"has_client"`
	HasUI           bool              `json:"has_ui"`
	HasCustomEntity bool              `json:"has_custom_entity"`
	HasCustomItems  bool              `json:"has_custom_items"`
	HasCustomBlocks bool              `json:"has_custom_blocks"`
	FileCount       map[string]uint32 `json:"file_count"` // 非 nil；分类名→计数，字典序 key
}

// systemEntry 对齐 RegisterSystem 三参产物 json!({"namespace","name","cls_path"})。字段顺序按
// serde_json BTreeMap 字典序（cls_path < name < namespace）。
type systemEntry struct {
	ClsPath   string `json:"cls_path"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// apisValue 对齐 json!({"components":[...],"methods":[...]})（字典序 c<m）。两字段必须非 nil。
type apisValue struct {
	Components []string `json:"components"`
	Methods    []string `json:"methods"`
}

// BuildTemplatesIndexReport 一次构建报告（非序列化）。只统计 client/ui/entity 三项。
type BuildTemplatesIndexReport struct {
	Total      uint32
	WithClient uint32
	WithUI     uint32
	WithEntity uint32
}

// buildTemplatesIndex 从 demoDir（每个子目录一个 mod）在 outputPath 生成 templates-index.json
// （对齐 build）。仅在致命失败（demo 目录不存在、写不出输出等）时返 error。
func buildTemplatesIndex(demoDir, outputPath string) (BuildTemplatesIndexReport, error) {
	if !isDir(demoDir) {
		return BuildTemplatesIndexReport{}, fmt.Errorf("demo dir does not exist: %s", demoDir)
	}
	var report BuildTemplatesIndexReport
	projects := findProjects(demoDir)
	entries := make([]OutputTemplateEntry, 0, len(projects))
	for _, proj := range projects {
		name, projPath := proj[0], proj[1]
		entry := analyzeProject(name, projPath, demoDir)
		if entry.HasClient {
			report.WithClient++
		}
		if entry.HasUI {
			report.WithUI++
		}
		if entry.HasCustomEntity {
			report.WithEntity++
		}
		entries = append(entries, entry)
	}
	report.Total = uint32(len(entries))

	// 顶层数组顺序 = findProjects 排序序（目录名升序），build 不二次排序。
	if parent := filepath.Dir(outputPath); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return BuildTemplatesIndexReport{}, fmt.Errorf("mkdir output parent: %w", err)
		}
	}
	data, err := marshalIndentNoEscape(entries)
	if err != nil {
		return BuildTemplatesIndexReport{}, fmt.Errorf("serialize templates: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return BuildTemplatesIndexReport{}, fmt.Errorf("write templates-index.json: %w", err)
	}
	return report, nil
}

// tagRule 是「项目名关键字 → tag 列表」一条规则（对齐 Python TAG_RULES）。
type tagRule struct {
	keyword string
	tags    []string
}

// tagRules 复刻上游 TAG_RULES（顺序无关，membership 决定输出；最终去重排序）。
var tagRules = []tagRule{
	{"传送门", []string{"维度", "传送门", "方块交互"}},
	{"方块", []string{"自定义方块"}},
	{"物品", []string{"自定义物品"}},
	{"生物", []string{"自定义生物", "实体"}},
	{"武器", []string{"武器", "战斗"}},
	{"3D武器", []string{"武器", "3D模型", "战斗"}},
	{"远程武器", []string{"武器", "远程", "战斗", "弹射物"}},
	{"盾牌", []string{"武器", "盾牌", "战斗"}},
	{"附魔", []string{"附魔", "物品"}},
	{"流体", []string{"自定义流体", "方块"}},
	{"农作物", []string{"农作物", "方块"}},
	{"熔炉", []string{"熔炉", "方块", "合成"}},
	{"精灵蛋", []string{"精灵蛋", "物品", "实体"}},
	{"书", []string{"自定义书", "物品", "UI"}},
	{"UI控件", []string{"UI", "控件"}},
	{"UI数据绑定", []string{"UI", "数据绑定"}},
	{"射击", []string{"射击", "战斗", "弹射物"}},
	{"寻路", []string{"寻路", "AI", "实体"}},
	{"绘制", []string{"自定义绘制", "渲染"}},
	{"破坏纹理", []string{"纹理", "渲染", "方块"}},
	{"粒子", []string{"粒子", "特效", "渲染"}},
	{"骨骼模型材质", []string{"骨骼模型", "材质", "渲染"}},
	{"后处理材质", []string{"后处理", "材质", "渲染"}},
	{"实体模型材质", []string{"实体模型", "材质", "渲染"}},
	{"玩家模型材质", []string{"玩家模型", "材质", "渲染"}},
	{"地形特征", []string{"地形", "生物群系", "世界生成"}},
	{"小地图", []string{"小地图", "UI", "渲染"}},
	{"成就", []string{"成就", "存档"}},
	{"云成就", []string{"成就", "云存档", "网络"}},
	{"指令", []string{"自定义指令", "命令"}},
	{"音乐", []string{"音乐", "音频"}},
	{"状态效果", []string{"状态效果", "实体"}},
	{"虚拟世界", []string{"虚拟世界", "维度"}},
	{"手柄", []string{"手柄", "UI", "输入"}},
	{"纸娃娃", []string{"纸娃娃", "渲染", "模型"}},
	{"微缩方块", []string{"微缩方块", "方块", "模型"}},
	{"方块网格体", []string{"方块网格体", "方块", "模型"}},
	{"模拟实体", []string{"实体", "物品"}},
	{"原生界面", []string{"UI", "原生界面修改"}},
	{"脚本入门", []string{"入门", "教程"}},
	{"联机大厅", []string{"联机大厅", "商品", "网络"}},
}

// generateTags 据项目名生成 tags；无关键字命中则回退 [project_name]（对齐 generate_tags）。
func generateTags(projectName string) []string {
	set := map[string]bool{}
	for _, rule := range tagRules {
		if strings.Contains(projectName, rule.keyword) {
			for _, t := range rule.tags {
				set[t] = true
			}
		}
	}
	if len(set) == 0 {
		return []string{projectName}
	}
	return sortedKeys(set)
}

// findProjects 找出 demo 项目目录（含名字带 behav 的子目录者）。返回 [(name, absPath)] 按目录名
// 升序（对齐 find_projects + paths.sort）。os.ReadDir 已按文件名升序，等价于同父目录的 PathBuf 排序。
func findProjects(demoDir string) [][2]string {
	out := [][2]string{}
	entries, err := os.ReadDir(demoDir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" {
			continue
		}
		projPath := filepath.Join(demoDir, name)
		if hasBehaviorSubdir(projPath) {
			out = append(out, [2]string{name, projPath})
		}
	}
	return out
}

// hasBehaviorSubdir 判项目内是否存在名字含 behav / 以 _behavior / beh 结尾的子目录（对齐
// has_behavior_subdir，全小写比较）。
func hasBehaviorSubdir(projPath string) bool {
	entries, err := os.ReadDir(projPath)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s := strings.ToLower(e.Name())
		if strings.Contains(s, "behav") || strings.HasSuffix(s, "_behavior") || strings.HasSuffix(s, "beh") {
			return true
		}
	}
	return false
}

// findPackDirs 找 behavior / resource pack 目录（各取遍历中第一个命中，对齐 find_pack_dirs 的
// else-if）。"" 表示未找到。注：Rust read_dir 顺序未定，Go os.ReadDir 按名升序 —— 多个候选时取名
// 字典序首个（项目通常单一 behav 目录，差异不显）。
func findPackDirs(projPath string) (string, string) {
	beh, res := "", ""
	entries, err := os.ReadDir(projPath)
	if err != nil {
		return "", ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(projPath, e.Name())
		s := strings.ToLower(e.Name())
		switch {
		case (strings.Contains(s, "behav") || strings.HasSuffix(s, "_behavior") || strings.HasSuffix(s, "beh")) && beh == "":
			beh = p
		case (strings.Contains(s, "resourc") || strings.HasSuffix(s, "_resource") || strings.HasSuffix(s, "res")) && res == "":
			res = p
		}
	}
	return beh, res
}

// findPythonFiles 递归收集 base 下全部 *.py（区分大小写，精确小写 "py"；对齐 find_python_files）。
// 栈式 DFS，不排序不去重。
func findPythonFiles(base string) []string {
	var out []string
	stack := []string{base}
	for len(stack) > 0 {
		d := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(d, e.Name())
			if e.IsDir() {
				stack = append(stack, p)
				continue
			}
			if rustExt(e.Name()) == "py" {
				out = append(out, p)
			}
		}
	}
	return out
}

// extCategory 是扩展名（小写）→ 分类映射（对齐 count_files 的 exts 表）。未命中 → "other"。
var extCategory = map[string]string{
	"py":          "py",
	"json":        "json",
	"png":         "png",
	"lang":        "lang",
	"material":    "material",
	"fragment":    "shader",
	"vertex":      "shader",
	"ogg":         "audio",
	"mcstructure": "structure",
}

// countFiles 递归统计 projPath 下各分类文件数（对齐 count_files）。扩展名大小写无关；无扩展名归
// "other"。返回非 nil map（空 → {}），key 序由 encoding/json 字典序保证（对齐 BTreeMap）。
func countFiles(projPath string) map[string]uint32 {
	counts := map[string]uint32{}
	stack := []string{projPath}
	for len(stack) > 0 {
		d := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				stack = append(stack, filepath.Join(d, e.Name()))
				continue
			}
			cat := extCategory[strings.ToLower(rustExt(e.Name()))]
			if cat == "" {
				cat = "other"
			}
			counts[cat]++
		}
	}
	return counts
}

// detectHasClient 判项目是否含客户端（py 路径含 "client"，或文件含 extraClientApi/clientApi/
// ClientSystem；对齐 detect_has_client）。
func detectHasClient(pyFiles []string) bool {
	for _, fp := range pyFiles {
		if strings.Contains(strings.ToLower(fp), "client") {
			return true
		}
		content, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		s := string(content)
		if strings.Contains(s, "extraClientApi") || strings.Contains(s, "clientApi") || strings.Contains(s, "ClientSystem") {
			return true
		}
	}
	return false
}

// detectHasUI 判项目是否含 UI（存在名为 "ui" 的子目录，或在 "ui" 目录内有 .json/.py 文件；对齐
// detect_has_ui）。栈式 DFS，ext 区分大小写。
func detectHasUI(projPath string) bool {
	stack := []string{projPath}
	for len(stack) > 0 {
		d := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		base := strings.ToLower(filepath.Base(d))
		var subdirs []string
		hasUIFile := false
		hasUIDirname := false
		for _, e := range entries {
			if e.IsDir() {
				subdirs = append(subdirs, filepath.Join(d, e.Name()))
				if strings.ToLower(e.Name()) == "ui" {
					hasUIDirname = true
				}
				continue
			}
			if base == "ui" {
				if ext := rustExt(e.Name()); ext == "json" || ext == "py" {
					hasUIFile = true
				}
			}
		}
		if (base == "ui" && hasUIFile) || hasUIDirname {
			return true
		}
		stack = append(stack, subdirs...)
	}
	return false
}

// detectHasCustomEntity 判 <behavior>/entities/ 下是否有 .json（对齐 detect_has_custom_entity）。
func detectHasCustomEntity(behaviorDir string) bool {
	if behaviorDir == "" {
		return false
	}
	entities := filepath.Join(behaviorDir, "entities")
	if !isDir(entities) {
		return false
	}
	return dirHasJSON(entities)
}

// detectHasCustomItems 判 <behavior>/*item*/ 下是否有 .json（对齐 detect_has_custom_items）。
func detectHasCustomItems(behaviorDir string) bool {
	if behaviorDir == "" {
		return false
	}
	entries, err := os.ReadDir(behaviorDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.Contains(strings.ToLower(e.Name()), "item") {
			if dirHasJSON(filepath.Join(behaviorDir, e.Name())) {
				return true
			}
		}
	}
	return false
}

// detectHasCustomBlocks 判 <behavior>/*block*/（排除 "blocks"）或 <behavior>/netease_blocks/ 下是否
// 有 .json（对齐 detect_has_custom_blocks）。
func detectHasCustomBlocks(behaviorDir string) bool {
	if behaviorDir == "" {
		return false
	}
	entries, err := os.ReadDir(behaviorDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := strings.ToLower(e.Name())
		if strings.Contains(n, "block") && n != "blocks" {
			if dirHasJSON(filepath.Join(behaviorDir, e.Name())) {
				return true
			}
		}
	}
	nb := filepath.Join(behaviorDir, "netease_blocks")
	if isDir(nb) {
		if dirHasJSON(nb) {
			return true
		}
	}
	return false
}

// dirHasJSON 判目录直接子项是否有扩展名 "json"（区分大小写，含名字带 .json 的子目录 —— 对齐 Rust
// 不过滤 is_dir 仅看 extension==Some("json")）。读失败 → false。
func dirHasJSON(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if rustExt(e.Name()) == "json" {
			return true
		}
	}
	return false
}

// extractEvents 扫 py 文件提取事件名（ListenForEvent 第三参 + 以 Event 结尾的字面量；对齐
// extract_events）。字典序去重，非 nil。
func extractEvents(pyFiles []string) []string {
	events := map[string]bool{}
	for _, fp := range pyFiles {
		content, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		s := string(content)
		scanListenForEvent(s, events)
		scanEventLiterals(s, events)
	}
	return sortedKeys(events)
}

// scanListenForEvent 字节扫 ListenForEvent(...)，跳前两个逗号分隔参数，捕第三个引号裸标识符
// （对齐 scan_listen_for_event）。
func scanListenForEvent(content string, out map[string]bool) {
	const needle = "ListenForEvent"
	n := len(content)
	i := 0
	for i+len(needle) < n {
		if !strings.HasPrefix(content[i:], needle) {
			i++
			continue
		}
		j := i + len(needle)
		for j < n && (content[j] == ' ' || content[j] == '\t') {
			j++
		}
		if j >= n || content[j] != '(' {
			i = j
			continue
		}
		j++
		// 在 depth 1 跳过两个逗号分隔参数（字符串内的括号/逗号不计）。
		depth := 1
		commas := 0
		broke := false
		for j < n && depth > 0 && commas < 2 {
			switch c := content[j]; {
			case c == '(':
				depth++
			case c == ')':
				depth--
				if depth == 0 {
					broke = true
				}
			case c == ',' && depth == 1:
				commas++
			case c == '"' || c == '\'':
				quote := c
				j++
				for j < n && content[j] != quote {
					j++
				}
			}
			if broke {
				break
			}
			j++
		}
		if commas < 2 || j >= n {
			i = max(j, i+1)
			continue
		}
		for j < n && (content[j] == ' ' || content[j] == '\t' || content[j] == '\n') {
			j++
		}
		if j >= n {
			break
		}
		if content[j] != '"' && content[j] != '\'' {
			i = j
			continue
		}
		quote := content[j]
		j++
		start := j
		for j < n && content[j] != quote {
			j++
		}
		if j >= n {
			break
		}
		if seg := content[start:j]; isIdent(seg) {
			out[seg] = true
		}
		i = j + 1
	}
}

// scanEventLiterals 扫全部引号字面量，收集以 "Event" 结尾的裸标识符（对齐 scan_event_literals）。
func scanEventLiterals(content string, out map[string]bool) {
	n := len(content)
	i := 0
	for i < n {
		b := content[i]
		if b == '"' || b == '\'' {
			quote := b
			i++
			start := i
			for i < n && content[i] != quote {
				i++
			}
			if i >= n {
				break
			}
			if seg := content[start:i]; strings.HasSuffix(seg, "Event") && isIdent(seg) {
				out[seg] = true
			}
			i++
		} else {
			i++
		}
	}
}

// excludeMethods 是 comp.Method 扫描里要排除的通用方法名（对齐 extract_apis 的 exclude_methods）。
var excludeMethods = map[string]bool{
	"Create":           true,
	"Get":              true,
	"Set":              true,
	"Init":             true,
	"Destroy":          true,
	"ListenForEvent":   true,
	"UnListenForEvent": true,
	"NotifyToClient":   true,
	"NotifyToServer":   true,
}

// extractAPIs 扫 py 文件提取 compFactory.CreateXxx 组件名 + comp.MethodName 方法名（对齐
// extract_apis）。components/methods 各自字典序去重，非 nil。
func extractAPIs(pyFiles []string) apisValue {
	components := map[string]bool{}
	methods := map[string]bool{}
	for _, fp := range pyFiles {
		content, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		s := string(content)
		scanCreateComp(s, "compFactory.Create", components)
		scanCreateComp(s, "GetEngineCompFactory().Create", components)
		scanCompMethods(s, methods, excludeMethods)
	}
	return apisValue{Components: sortedKeys(components), Methods: sortedKeys(methods)}
}

// scanCreateComp 在 prefix 之后捕获到 '(' 前的标识符（对齐 scan_create_comp）。
func scanCreateComp(content, prefix string, out map[string]bool) {
	n := len(content)
	start := 0
	for {
		rel := strings.Index(content[start:], prefix)
		if rel < 0 {
			break
		}
		abs := start + rel + len(prefix)
		j := abs
		for j < n && isASCIIWord(content[j]) {
			j++
		}
		if j > abs && j < n && content[j] == '(' {
			if name := content[abs:j]; name != "" {
				out[name] = true
			}
		}
		start = max(j, start+1)
	}
}

// scanCompMethods 捕 comp./Comp. 后的大写起首标识符 + '('（词边界 + 排除通用名；对齐
// scan_comp_methods）。
func scanCompMethods(content string, out, exclude map[string]bool) {
	n := len(content)
	i := 0
	for i+1 < n {
		if content[i] != '.' {
			i++
			continue
		}
		beforeEnd := i
		beforeStart := beforeEnd - 4
		if beforeStart < 0 {
			beforeStart = 0
		}
		pre := content[beforeStart:beforeEnd]
		var isWordBefore bool
		if beforeStart == 0 {
			isWordBefore = true
		} else {
			prev := content[beforeStart-1]
			isWordBefore = !(isASCIIAlnum(prev) || prev == '_')
		}
		endsWithComp := strings.HasSuffix(pre, "comp") || strings.HasSuffix(pre, "Comp")
		if !(isWordBefore && endsWithComp) {
			i++
			continue
		}
		j := i + 1
		for j < n && (content[j] == ' ' || content[j] == '\t') {
			j++
		}
		if j >= n || !isASCIIUpper(content[j]) {
			i++
			continue
		}
		start := j
		for j < n && isASCIIWord(content[j]) {
			j++
		}
		if j > start && j < n && content[j] == '(' {
			if name := content[start:j]; !exclude[name] {
				out[name] = true
			}
		}
		i = max(j, i+1)
	}
}

// extractRegisterSystems 扫 py 文件提取 RegisterSystem 三参（对齐 extract_register_systems）。
// 保遍历序，不排序不去重，非 nil（空 → []）。
func extractRegisterSystems(pyFiles []string) []systemEntry {
	systems := []systemEntry{}
	for _, fp := range pyFiles {
		content, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		scanRegisterSystem(string(content), &systems)
	}
	return systems
}

// scanRegisterSystem 字节扫 RegisterSystem(...)，解析恰好三个引号参数（对齐 scan_register_system）。
func scanRegisterSystem(content string, out *[]systemEntry) {
	const needle = "RegisterSystem"
	n := len(content)
	i := 0
	for i+len(needle) < n {
		if !strings.HasPrefix(content[i:], needle) {
			i++
			continue
		}
		j := i + len(needle)
		for j < n && (content[j] == ' ' || content[j] == '\t') {
			j++
		}
		if j >= n || content[j] != '(' {
			i = j
			continue
		}
		j++
		var args []string
		local := j
		for k := 0; k < 3; k++ {
			for local < n && (content[local] == ' ' || content[local] == '\t' || content[local] == ',' || content[local] == '\n') {
				local++
			}
			if local >= n {
				break
			}
			q := content[local]
			if q != '"' && q != '\'' {
				break
			}
			local++
			start := local
			for local < n && content[local] != q {
				local++
			}
			if local >= n {
				break
			}
			if seg := content[start:local]; utf8.ValidString(seg) {
				args = append(args, seg)
			}
			local++
		}
		if len(args) == 3 {
			*out = append(*out, systemEntry{Namespace: args[0], Name: args[1], ClsPath: args[2]})
		}
		i = max(local, i+1)
	}
}

// extractModName 解析 mod 名：在 behaviorDir 下找 modMain.py 的 @Mod.Binding(name="...")，否则回退
// modConfig.py 的 ModName/ModType（对齐 extract_mod_name）。
func extractModName(behaviorDir string) string {
	stack := []string{behaviorDir}
	for len(stack) > 0 {
		d := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(d, e.Name())
			if e.IsDir() {
				stack = append(stack, p)
				continue
			}
			if e.Name() == "modMain.py" {
				content, err := os.ReadFile(p)
				if err != nil {
					continue
				}
				if name, ok := findModBinding(string(content)); ok {
					return name
				}
				if name, ok := findModNameInConfig(behaviorDir); ok {
					return name
				}
			}
		}
	}
	if name, ok := findModNameInConfig(behaviorDir); ok {
		return name
	}
	return ""
}

// findModBinding 在 @Mod.Binding(...) 括号内找 name="X"（X 须裸标识符；对齐 find_mod_binding）。
func findModBinding(content string) (string, bool) {
	const needle = "@Mod.Binding"
	idx := strings.Index(content, needle)
	if idx < 0 {
		return "", false
	}
	n := len(content)
	j := idx + len(needle)
	for j < n && (content[j] == ' ' || content[j] == '\t') {
		j++
	}
	if j >= n || content[j] != '(' {
		return "", false
	}
	j++
	for j < n && content[j] != ')' {
		if strings.HasPrefix(content[j:], "name") {
			k := j + 4
			for k < n && (content[k] == ' ' || content[k] == '\t') {
				k++
			}
			if k >= n || content[k] != '=' {
				j++
				continue
			}
			k++
			for k < n && (content[k] == ' ' || content[k] == '\t') {
				k++
			}
			if k < n && (content[k] == '"' || content[k] == '\'') {
				q := content[k]
				k++
				start := k
				for k < n && content[k] != q {
					k++
				}
				if seg := content[start:k]; isIdent(seg) {
					return seg, true
				}
			}
			return "", false
		}
		j++
	}
	return "", false
}

// findModNameInConfig 栈式 DFS 找 modConfig.py，扫 ModName / ModType 赋值（对齐
// find_mod_name_in_config）。
func findModNameInConfig(start string) (string, bool) {
	stack := []string{start}
	for len(stack) > 0 {
		d := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(d, e.Name())
			if e.IsDir() {
				stack = append(stack, p)
				continue
			}
			if e.Name() == "modConfig.py" {
				content, err := os.ReadFile(p)
				if err != nil {
					continue
				}
				s := string(content)
				if name, ok := scanAssignment(s, "ModName"); ok {
					return name, true
				}
				if name, ok := scanAssignment(s, "ModType"); ok {
					return name, true
				}
			}
		}
	}
	return "", false
}

// scanAssignment 找 `var = "X"`（var 须词边界，X 非空，可含任意字符；对齐 scan_assignment）。
func scanAssignment(content, varName string) (string, bool) {
	n := len(content)
	pos := 0
	for {
		rel := strings.Index(content[pos:], varName)
		if rel < 0 {
			break
		}
		i := pos + rel
		beforeOK := i == 0 || !(isASCIIAlnum(content[i-1]) || content[i-1] == '_')
		if !beforeOK {
			pos = i + len(varName)
			continue
		}
		j := i + len(varName)
		if j < n && (isASCIIAlnum(content[j]) || content[j] == '_') {
			pos = j
			continue
		}
		for j < n && (content[j] == ' ' || content[j] == '\t') {
			j++
		}
		if j >= n || content[j] != '=' {
			pos = j
			continue
		}
		j++
		for j < n && (content[j] == ' ' || content[j] == '\t') {
			j++
		}
		if j < n && (content[j] == '"' || content[j] == '\'') {
			q := content[j]
			j++
			start := j
			for j < n && content[j] != q {
				j++
			}
			if seg := content[start:j]; utf8.ValidString(seg) && seg != "" {
				return seg, true
			}
		}
		pos = j
	}
	return "", false
}

// analyzeProject 对单个 demo 项目跑全套分析（对齐 analyze_project）。
func analyzeProject(name, projPath, demoRoot string) OutputTemplateEntry {
	behaviorDir, resourceDir := findPackDirs(projPath)
	var pyFiles []string
	modName := ""
	if behaviorDir != "" {
		pyFiles = findPythonFiles(behaviorDir)
		modName = extractModName(behaviorDir)
	}

	behName := ""
	if behaviorDir != "" {
		behName = filepath.Base(behaviorDir)
	}
	resName := ""
	if resourceDir != "" {
		resName = filepath.Base(resourceDir)
	}

	return OutputTemplateEntry{
		ID:              name,
		Path:            makeRelPath(projPath, demoRoot),
		ModName:         modName,
		BehaviorPack:    behName,
		ResourcePack:    resName,
		Tags:            generateTags(name),
		Systems:         extractRegisterSystems(pyFiles),
		EventsUsed:      extractEvents(pyFiles),
		APIs:            extractAPIs(pyFiles),
		HasClient:       detectHasClient(pyFiles),
		HasUI:           detectHasUI(projPath),
		HasCustomEntity: detectHasCustomEntity(behaviorDir),
		HasCustomItems:  detectHasCustomItems(behaviorDir),
		HasCustomBlocks: detectHasCustomBlocks(behaviorDir),
		FileCount:       countFiles(projPath),
	}
}

// makeRelPath 生成相对路径 "<demoRoot 末段>/<rel>"（正斜杠）；strip 失败回退整条 projPath
// （对齐 analyze_project 的 strip_prefix 分支）。
func makeRelPath(projPath, demoRoot string) string {
	rel, err := filepath.Rel(demoRoot, projPath)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Base(demoRoot) + "/" + filepath.ToSlash(rel)
	}
	return filepath.ToSlash(projPath)
}

// ── 共享小工具 ───────────────────────────────────────────────────────────

// sortedKeys 把 set 的键收集为字典序升序切片（复刻 BTreeSet → Vec）。返回非 nil（空 → []）。
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// isDir 判路径是否目录（os.Stat 跟随符号链接，对齐 Rust Path::is_dir）。
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// rustExt 复刻 Rust Path::extension：取文件名最后一个 '.' 之后部分；无内嵌点 / 以 '.' 开头且无其他
// 点 / 文件名为空 → 返回 ""。返回值含大小写原样（调用方按需 ToLower）。
func rustExt(name string) string {
	base := name
	if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
		base = base[i+1:]
	}
	dot := strings.LastIndexByte(base, '.')
	if dot <= 0 { // dot<0：无点；dot==0：以 '.' 开头（如 ".py"）→ 无扩展
		return ""
	}
	return base[dot+1:]
}

func isASCIIWord(c byte) bool  { return isASCIIAlnum(c) || c == '_' }
func isASCIIUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isASCIIAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isIdent 判 s 是否非空、合法 UTF-8、且每个字符都是 alnum 或 '_'（复刻 from_utf8 +
// !is_empty + chars().all(is_alphanumeric || '_')；is_alphanumeric 为 Unicode 语义）。
func isIdent(s string) bool {
	if s == "" || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_') {
			return false
		}
	}
	return true
}
