// Package webapi 是 P7 的「Web 网关」：一个独立于 orchestrator（127.0.0.1 + MCP）的
// HTTP server，bind **0.0.0.0:固定端口**，让手机/平板经局域网访问与桌面互通的纯 Web 界面。
//
// M1（本单元，网关地基）只做：
//   - 0.0.0.0 监听 + 同源托管内嵌的 frontend/dist（SPA）
//   - 跨重启稳定的 Bearer token（cryptox 加密落 app_settings.web_gateway_token_enc）
//   - 配对：GET /pair?token=xxx → 校验 token → 下发 httpOnly Session Cookie → 重定向 /
//   - 鉴权：除 /api/health 与 /pair 外，未持有效 cookie 一律 401（含静态 SPA）
//   - GET /api/health 无鉴权探活
//
// 真正的会话编排 REST（/api/launch、/api/sessions/*）与 /ws 实时广播在 M3 接入。
// 安全铁律：本网关与 orchestrator 物理隔离（独立 listener / 独立 token / 不改 Accept 头）。
// 明文密钥红线（provider.ResolveLaunchEnv 等解密出口）永不挂任何路由。
package webapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
	"xkpane/internal/utils/logx"

	"gorm.io/gorm"
)

// DefaultPort 是 Web 网关的固定监听端口（手机书签需稳定，故不取随机端口）。
const DefaultPort = 9317

// cookieName 是配对后下发的 httpOnly Session Cookie 名。
const cookieName = "xkpane_web_session"

// cookiePurpose 是 HMAC 的固定上下文串（防 cookie 被挪作他用）。
const cookiePurpose = "xkpane-paired-v1"

// Gateway 是 Web 网关运行态。生命周期由 app 装配处管（Start 后台起 / Stop 优雅关），
// 非 Wails service —— 前端访问面只经 webapi.Service.GetAccessInfo（见 service.go）。
type Gateway struct {
	holder *storage.Holder
	assets fs.FS // 已 fs.Sub 到 frontend/dist 根的内嵌前端资源
	hub    *hub  // WS 广播中枢（远程事件镜像，§6.1）

	// deps 是远程 REST/launch 的下游能力（app 装配处经 AttachServices 注入，orchestrator 建好后）。
	// 未注入时相关端点返回 503（fail-closed）。
	depsMu sync.RWMutex
	deps   ServiceDeps

	mu    sync.RWMutex
	port  int
	token string // 解密后的明文 token（内存态）；未启动为 ""
	srv   *http.Server
}

// AccessInfo 是桌面端「设置 → Web 访问」卡片渲染配对二维码所需信息。
// PrimaryURL / URLs 内嵌明文 token —— 仅下发给本地可信的桌面 webview（配对二维码），
// 不会下发给远程浏览器（远程走 cookie，token 不进其 JS）。
type AccessInfo struct {
	Port       int      `json:"port"`
	URLs       []string `json:"urls"`       // 每个局域网 IPv4 一条 http://<ip>:<port>/pair?token=...
	PrimaryURL string   `json:"primaryUrl"` // URLs 首个，二维码默认用它
}

// NewGateway 构造一个未启动的 Web 网关。assets 须已 fs.Sub 到 frontend/dist 根。
func NewGateway(holder *storage.Holder, assets fs.FS) *Gateway {
	return &Gateway{holder: holder, assets: assets, hub: newHub()}
}

// AttachServices 注入远程 REST/launch 的下游能力（app 装配处在 orchestrator 建好后调用）。
func (g *Gateway) AttachServices(deps ServiceDeps) {
	g.depsMu.Lock()
	g.deps = deps
	g.depsMu.Unlock()
}

func (g *Gateway) services() ServiceDeps {
	g.depsMu.RLock()
	defer g.depsMu.RUnlock()
	return g.deps
}

// RouteEvent 是「组合双路 emit」的远程路径（app.go 的 mirrorEmit 调用）：把会话事件转发给所有
// 浏览器连接。只转发镜像白名单内的事件（终端输出/状态/退出 + 编排 launch），其余（time /
// localhistory:changed 等本地/无关事件）丢弃，避免无谓占用 WS 带宽。**非阻塞**（hub.broadcast
// 满即剔除慢端），绝不反压回 terminal/PTY。
func (g *Gateway) RouteEvent(name string, payload any) {
	if !mirroredEvents[name] {
		return
	}
	g.hub.broadcast(name, payload)
}

// Start 加载/生成稳定 token，bind 0.0.0.0:DefaultPort 起 HTTP server（后台 goroutine，非阻塞）。
// 返回 error 由调用方 fail-open（Web 访问不可用，不阻断桌面应用启动）。
func (g *Gateway) Start() error {
	token, err := g.ensureToken()
	if err != nil {
		return fmt.Errorf("webapi ensure token: %w", err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", DefaultPort))
	if err != nil {
		return fmt.Errorf("webapi listen 0.0.0.0:%d: %w", DefaultPort, err)
	}

	srv := &http.Server{Handler: g.buildMux()}
	g.mu.Lock()
	g.port = DefaultPort
	g.token = token
	g.srv = srv
	g.mu.Unlock()

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.For("webapi").Error("serve stopped", "err", err)
		}
	}()

	urls := g.pairingURLs(DefaultPort, token)
	logx.For("webapi").Info("web gateway started",
		"port", DefaultPort,
		"urls", strings.Join(urls, ", "))
	return nil
}

// Stop 优雅关闭（3s 上限）。未启动时 no-op。
func (g *Gateway) Stop() error {
	g.mu.RLock()
	srv := g.srv
	g.mu.RUnlock()
	if srv == nil {
		return nil
	}
	// http.Server.Shutdown 自带 ctx 超时；这里给 3s。
	done := make(chan error, 1)
	go func() { done <- srv.Close() }()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		return srv.Close()
	}
}

// AccessInfo 返回桌面卡片配对所需信息（端口 + 各局域网 IPv4 的配对 URL）。未启动返回 error。
func (g *Gateway) accessInfo() (AccessInfo, error) {
	g.mu.RLock()
	port, token := g.port, g.token
	g.mu.RUnlock()
	if port == 0 {
		return AccessInfo{}, errors.New("web gateway 未启动")
	}
	urls := g.pairingURLs(port, token)
	info := AccessInfo{Port: port, URLs: urls}
	if len(urls) > 0 {
		info.PrimaryURL = urls[0]
	}
	return info, nil
}

// pairingURLs 为每个局域网 IPv4 生成一条配对 URL（内嵌 token，供桌面二维码）。
func (g *Gateway) pairingURLs(port int, token string) []string {
	ips := lanIPv4s()
	urls := make([]string, 0, len(ips))
	for _, ip := range ips {
		urls = append(urls, fmt.Sprintf("http://%s:%d/pair?token=%s", ip, port, token))
	}
	return urls
}

// ensureToken 读 app_settings 单行的 web_gateway_token_enc；为空或解密失败则生成新 token、
// 加密回写。返回明文 token。
func (g *Gateway) ensureToken() (string, error) {
	db := g.holder.Current().GORM
	var row storage.AppSettings
	err := db.First(&row, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 正常路径下 storage.Open 已 ensureSingletonSettings；此处兜底建行。
		row = storage.AppSettings{ID: 1, ThemeChoice: "system", LocaleChoice: "auto", NotificationEnabled: true}
		if e := db.Create(&row).Error; e != nil {
			return "", e
		}
	} else if err != nil {
		return "", err
	}

	if row.WebGatewayTokenEnc != "" {
		if tok, derr := cryptox.DecryptString(row.WebGatewayTokenEnc); derr == nil && tok != "" {
			return tok, nil
		}
		// 解密失败（如主密钥变更）→ 重新生成，旧 cookie 自然失效需重新配对。
		logx.For("webapi").Warn("decrypt stored token failed, regenerating")
	}

	tok := genToken()
	enc, err := cryptox.EncryptString(tok)
	if err != nil {
		return "", err
	}
	if err := db.Model(&storage.AppSettings{}).Where("id = 1").
		Update("web_gateway_token_enc", enc).Error; err != nil {
		return "", err
	}
	return tok, nil
}

// sessionCookieValue 是配对成功后下发的 cookie 值 = HMAC-SHA256(token, purpose)。
// 无 token 不可伪造；token 稳定故 cookie 跨重启有效。
func (g *Gateway) sessionCookieValue() string {
	mac := hmac.New(sha256.New, []byte(g.currentToken()))
	mac.Write([]byte(cookiePurpose))
	return hex.EncodeToString(mac.Sum(nil))
}

// validCookie 常量时间校验请求是否持有效 Session Cookie。
func (g *Gateway) validCookie(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return false
	}
	want := g.sessionCookieValue()
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(want)) == 1
}

func (g *Gateway) currentToken() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.token
}

// genToken 生成 48-hex 随机 token（24 字节 crypto/rand）。
func genToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("webapi: crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// lanIPv4s 枚举本机非回环、已启用网卡上的私有 IPv4（192.168/10/172.16-31）。
// 手机经其中之一访问网关；多网卡（有线+无线+虚拟）会返回多条，桌面卡片可让用户挑。
func lanIPv4s() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			ip4 := ip.To4()
			if ip4 == nil || !ip4.IsPrivate() {
				continue
			}
			out = append(out, ip4.String())
		}
	}
	return out
}
