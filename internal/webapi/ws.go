package webapi

// ws.go：GET /ws 的升级与单连接写循环（coder/websocket）。已过 requirePaired 闸门（持有效
// httpOnly cookie）才会走到这里。同源校验由 websocket.Accept 默认行为兜底（request host 总被
// 授权、跨源 Origin 拒绝 → 等价 P7 §D7 同源策略，无需 CORS/自定义 CheckOrigin）。
//
// 连接是**纯下行推送**（服务器 → 浏览器的事件镜像）：用 CloseRead 持续读并丢弃入站帧、
// 处理 ping/pong 与关闭帧；写循环从该连接的有界 send 队列取消息发出。M6 起浏览器若要回传
// 输入将另走 HTTP（POST /api/sessions/{sid}/input），不在此 WS 上行。

import (
	"context"
	"net/http"
	"time"

	"xkpane/internal/utils/logx"

	"github.com/coder/websocket"
)

// wsWriteTimeout 是单帧写超时上限（慢端写阻塞的硬上限；超时即关连接，配合 hub 剔除语义）。
const wsWriteTimeout = 10 * time.Second

// handleWS 升级连接、登记进 hub，跑写循环直到对端断开 / 慢端被剔除 / server 关闭。
func (g *Gateway) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// 默认同源校验：request host 总授权、跨源 Origin 拒绝（不设 OriginPatterns / 不开
		// InsecureSkipVerify）。手机从 http://<ip>:9317 加载页面 → 同源连接放行。
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		// Accept 失败已自行写过响应（如 403 跨源）；仅记一笔。
		logx.For("webapi").Debug("ws accept failed", "err", err)
		return
	}

	client := newWSClient()
	g.hub.add(client)
	defer g.hub.remove(client)

	// CloseRead：持续读并丢弃入站帧（处理 ping/pong/close），返回的 ctx 在对端断开时取消。
	ctx := conn.CloseRead(r.Context())

	for {
		select {
		case <-ctx.Done():
			// 对端断开 / server 请求关闭 → 正常收尾。
			_ = conn.Close(websocket.StatusNormalClosure, "")
			return
		case <-client.closed:
			// 被 hub 剔除（慢端：send 队列满）→ 用 policy violation 关闭，客户端据此重连。
			_ = conn.Close(websocket.StatusPolicyViolation, "slow consumer")
			return
		case msg := <-client.send:
			wctx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
			err := conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				// 写失败（断开/超时）→ 退出，defer remove 清理。
				return
			}
		}
	}
}
