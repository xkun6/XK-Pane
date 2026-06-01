package webapi

// Service 是 Web 网关暴露给桌面前端的**薄 Wails 门面**：只有 GetAccessInfo 一个方法，
// 供「设置 → Web 访问」卡片拿配对 URL 渲染二维码。生命周期（Start/Stop）不在此暴露——
// 由 app 装配处直接管 *Gateway，避免把 Start/Stop 也绑给前端。
type Service struct {
	gw *Gateway
}

// NewService 包一个已构造的 *Gateway 成 Wails service。
func NewService(gw *Gateway) *Service {
	return &Service{gw: gw}
}

// GetAccessInfo 返回手机配对所需信息（端口 + 各局域网 IPv4 的配对 URL，内嵌 token）。
// 仅桌面端调用（渲染二维码）；网关未启动返回 error。
func (s *Service) GetAccessInfo() (AccessInfo, error) {
	return s.gw.accessInfo()
}
