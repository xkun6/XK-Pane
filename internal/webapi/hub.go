package webapi

// hub.go：WebSocket 事件广播中枢（P7 §6.1 远程路径）。后端 emit 的会话事件经 RouteEvent →
// hub.broadcast 扇出到所有已配对的浏览器连接。**铁律：绝不反压回调用方**——每个连接持一个
// 有界 send 队列，满了即「剔除该连接」（关闭 → 客户端重连，M4 起重连重放），而不是阻塞
// broadcast（那会经 mirrorEmit 反压回 terminal.batchLoop/PTY，一个卡顿手机拖垮整机终端）。
//
// 与桌面 webview 路径（app.Event.Emit，进程内同步）严格隔离：broadcast 只动远程连接，
// 桌面零感知零回归。

import (
	"encoding/json"
	"sync"

	"xkpane/internal/utils/logx"
)

// clientSendBuffer 是单个浏览器连接的待发队列容量。够吸收正常抖动；持续填满 = 该端跟不上
// （网络慢/页面卡），按铁律剔除而非拖慢全局。
const clientSendBuffer = 256

// wsEnvelope 是发给浏览器的统一帧：{event, data}。浏览器 transport 按 event 名扇出，data 是
// 与桌面 Wails 事件同构的 payload（events.*Payload 的 json 形态，camelCase）。
type wsEnvelope struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// wsClient 是一条浏览器 WS 连接在 hub 中的登记：一个有界 send 队列 + 一次性关闭信号。
type wsClient struct {
	send   chan []byte
	closed chan struct{}
	once   sync.Once
}

func newWSClient() *wsClient {
	return &wsClient{
		send:   make(chan []byte, clientSendBuffer),
		closed: make(chan struct{}),
	}
}

// kill 幂等地标记该连接应被关闭（writer goroutine 据此退出并 removeClient）。
func (c *wsClient) kill() {
	c.once.Do(func() { close(c.closed) })
}

// hub 持有所有活跃浏览器连接。线程安全。
type hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func newHub() *hub {
	return &hub{clients: make(map[*wsClient]struct{})}
}

func (h *hub) add(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *hub) remove(c *wsClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.kill()
}

// clientCount 返回当前连接数（探活/日志/测试用）。
func (h *hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// broadcast 把一条事件投递到所有连接的有界队列。**非阻塞**：队列满即剔除该连接（kill），
// 绝不等待——保证 mirrorEmit/terminal 路径永不被某个慢端拖住。
func (h *hub) broadcast(event string, payload any) {
	msg, err := json.Marshal(wsEnvelope{Event: event, Data: payload})
	if err != nil {
		logx.For("webapi").Warn("ws marshal failed", "event", event, "err", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		case <-c.closed:
			// 该连接已在关闭中，跳过。
		default:
			// 队列满 = 慢端：标记关闭（writer 退出后 removeClient），不阻塞 broadcast。
			c.kill()
		}
	}
}
