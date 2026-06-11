package bedrock

// service.go：Bedrock 查询子系统的零状态句柄（对齐 service.rs BedrockQueryService）。
// 所有数据都在进程级可失效缓存（loader/index）里，首次查询时经 getIndices() 惰性构建；
// 本结构只是给下游 orchestrator / 测试的注入点
// （查询本身无状态、无需 storage holder；OTA 状态由 bedrockdocsync 包另持）。

// Service 是 Bedrock 查询服务句柄。构造廉价、无 I/O。
type Service struct{}

// New 构造一个查询服务句柄。
func New() *Service { return &Service{} }

// ── Phase 2：高频查询工具（10）→ 委托 search_*.go ───────────────────────

func (s *Service) SearchAPI(name string) (string, error) { return searchAPI(name) }

func (s *Service) GetAPI(name string, side *string) (string, error) { return getAPI(name, side) }

func (s *Service) SearchEvent(name string) (string, error) { return searchEvent(name) }

func (s *Service) GetEvent(name string, side *string) (string, error) { return getEvent(name, side) }

func (s *Service) ValidateCall(name, side string) (string, error) { return validateCall(name, side) }

func (s *Service) APIStats() (string, error) { return apiStats() }

func (s *Service) ListCategories() (string, error) { return listCategories() }

func (s *Service) ListApisByCategory(category string) (string, error) {
	return listApisByCategory(category)
}

func (s *Service) SearchComponent(query, componentType string) (string, error) {
	return searchComponent(query, componentType)
}

func (s *Service) GetComponent(componentID string) (string, error) { return getComponent(componentID) }

// ── Phase 3：指南 / 模板 / 标准工具（12）→ 委托 search_guide/template + standards ──────

func (s *Service) SearchGuide(keyword string, source *string) (string, error) {
	return searchGuide(keyword, source)
}

func (s *Service) ReadGuide(path string, maxLines *uint32) (string, error) {
	return readGuide(path, maxLines)
}

func (s *Service) MatchTemplate(tags []string, keyword string) (string, error) {
	return matchTemplate(tags, keyword)
}

func (s *Service) GetTemplateInfo(templateID string) (string, error) {
	return templateInfo(templateID)
}

func (s *Service) GetTemplateFile(templateID, filePath string) (string, error) {
	return templateFile(templateID, filePath)
}

func (s *Service) GetStandard(name string) (string, error) { return getStandard(name) }

func (s *Service) GetExpertGuide() (string, error) { return expertGuide() }

func (s *Service) GetReviewRules() (string, error) { return reviewRules() }

func (s *Service) GetOptimizationRules() (string, error) { return optimizationRules() }

func (s *Service) GetJSONSpec(contentType *string) (string, error) { return jsonSpec(contentType) }

func (s *Service) BedrockHelp() (string, error) { return helpText() }

func (s *Service) ListModPacks() (string, error) { return listModPacks() }

// ── Phase 4：静态代码审查（1）→ 委托 review ─────────────────────────────

// ReviewCode ← pe_review_code。filename 默认在 service 层解析（对齐 service.rs:150）。
func (s *Service) ReviewCode(code string, filename *string) (string, error) {
	fn := "unknown.py"
	if filename != nil {
		fn = *filename
	}
	return reviewCode(code, fn)
}

// ── Phase 5：文件系统 / JSON 校验器（3）→ 委托 validate ──────────────────

func (s *Service) ValidateJSONDefinitions(addonDir string) (string, error) {
	return validateJSONDefinitions(addonDir)
}

func (s *Service) ValidateAddonAssets(addonDir string) (string, error) {
	return validateAddonAssets(addonDir)
}

func (s *Service) ValidateTest(testJSON string) (string, error) { return validateTest(testJSON) }
