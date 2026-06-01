package bedrock

import "xkpane/internal/utils/logx"

// service.go：Bedrock 查询子系统的零状态句柄（对齐 service.rs BedrockQueryService）。
// 所有数据都在进程级可失效缓存（loader/index）里，本结构只是给下游 orchestrator / 测试的注入点
// （查询本身无状态、无需 storage holder；OTA 状态由 bedrockdocsync 包另持）。

// Service 是 Bedrock 查询服务句柄。构造廉价、无 I/O。
type Service struct{}

// New 构造一个查询服务句柄。
func New() *Service { return &Service{} }

// Warm 首调即惰性构建全部索引（可用作启动预热）。对齐 BedrockQueryService::warm。
func (s *Service) Warm() error {
	_, err := getIndices()
	if err != nil {
		logx.For("bedrock").Warn("warm indices failed", "err", err)
	}
	return err
}

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
