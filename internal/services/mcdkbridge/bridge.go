package mcdkbridge

// bridge.go — TCP server，等游戏内 Python 桥模组反向连入。移植自 bridge.rs，多客户端并存为 XK 扩展。
//
// 工作模式（参考游戏内 IPCSystem.py）：
//   - Mod 是纯接收方：只读不写，不主动握手（新版按 §hello 补一条反向帧）。
//   - 我们 bind 127.0.0.1:0 拿 ephemeral port，accept 全部连入的 client，多开 MC PE 并存（不互踢）。
//   - hello 归属：mod 连上后先发 PacketHello（payload=runId），是本连接的第一帧；XK-Pane 据此把
//     该连接后续的 PacketLogLine 精确归到对应 run。老模组不发 hello，runID 按空串处理（单开兜底见
//     mcpe.Service.handleBridgeLog）。
//   - 后台 read goroutine 除检测 EOF（mod 退出 → 从 map 摘除该 client）外，还负责解 hello/日志帧。
//   - 「最新连接者」语义（newestLocked）：单开场景下 Status()/SendPacket 沿用旧的隐式目标——
//     不带 runID 时操作最新连入的 client，兼容历史调用方。
//
// 隐藏坑（务必 bind 127.0.0.1，不要 ::1 / 0.0.0.0）：游戏内 Python connect(("localhost", port))，
// 若 listener 落到 IPv6（::1）而 localhost 优先解析到它之外的地址会连不上。cc-pane 与此一致。

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"sync"
	"time"

	"xkpane/internal/utils/logx"
)

// ClientStatus 当前 active client 的快照状态（未连接时 RemoteAddr 空、ConnectedAtMs 0）。
type ClientStatus struct {
	Connected     bool
	RemoteAddr    string
	ConnectedAtMs int64
}

type activeClient struct {
	id            uint64
	conn          net.Conn
	connectedAtMs int64
	remoteAddr    string
	runID         string // hello 帧回传；未收到 hello（老模组）为空
}

// Service 是 MCDK 调试桥 TCP server。
type Service struct {
	mu         sync.Mutex
	listener   net.Listener
	clients    map[uint64]*activeClient // 多开并存，key=连接自增 id
	nextID     uint64
	listenPort int
	log        *slog.Logger

	logHandlerMu sync.RWMutex
	logHandler   func(runID, line string) // 游戏内桥模组反向日志帧（PacketLogLine）的接收回调，可 nil
}

// SetLogHandler 注入反向日志帧回调（mcpe.Service 装配时调）。每收到一帧 PacketLogLine
// 就以 (runID, payload字符串) 回调一次；runID 取自本连接的 hello 帧（老模组为空串）。
// 回调在 readDrain goroutine 上同步执行，实现方应自己不阻塞。
func (s *Service) SetLogHandler(fn func(runID, line string)) {
	s.logHandlerMu.Lock()
	s.logHandler = fn
	s.logHandlerMu.Unlock()
}

// New 构造未启动的桥（端口在 Start 后才确定）。
func New() *Service {
	return &Service{nextID: 1, clients: map[uint64]*activeClient{}, log: logx.For("mcdk-bridge")}
}

// Start bind 127.0.0.1:0 并起 accept goroutine。失败返回带 MCDK_BRIDGE_BIND_FAILED 的 CodedError。
func (s *Service) Start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return coded(CodeBridgeBindFailed, fmt.Sprintf("bind 127.0.0.1:0 failed: %v", err))
	}
	port := ln.Addr().(*net.TCPAddr).Port
	s.mu.Lock()
	s.listener = ln
	s.listenPort = port
	s.mu.Unlock()
	go s.acceptLoop(ln)
	return nil
}

// Stop 关闭 listener 与全部 client（XK 生命周期约定；cc-pane 无此方法，桥随 app 常驻）。
func (s *Service) Stop() error {
	s.mu.Lock()
	ln := s.listener
	old := s.clients
	s.listener = nil
	s.clients = map[uint64]*activeClient{}
	s.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range old {
		_ = c.conn.Close()
	}
	return nil
}

// Port 返回监听端口（未 Start 时为 0）。
func (s *Service) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listenPort
}

// newestLocked 返回 id 最大（最新连入）的 client；无 client 时 nil。调用方须已持 s.mu。
func (s *Service) newestLocked() *activeClient {
	var newest *activeClient
	for _, c := range s.clients {
		if newest == nil || c.id > newest.id {
			newest = c
		}
	}
	return newest
}

// Status 返回最新连入 client 的连接快照（单开语义，兼容历史调用方）。
func (s *Service) Status() ClientStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c := s.newestLocked(); c != nil {
		return ClientStatus{Connected: true, RemoteAddr: c.remoteAddr, ConnectedAtMs: c.connectedAtMs}
	}
	return ClientStatus{}
}

// StatusForRun 返回 runID 匹配的最新 client 的连接快照；无匹配返回零值。
func (s *Service) StatusForRun(runID string) ClientStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	var newest *activeClient
	for _, c := range s.clients {
		if c.runID == runID && (newest == nil || c.id > newest.id) {
			newest = c
		}
	}
	if newest != nil {
		return ClientStatus{Connected: true, RemoteAddr: newest.remoteAddr, ConnectedAtMs: newest.connectedAtMs}
	}
	return ClientStatus{}
}

// ClientRunIDs 按连接 id 升序返回当前全部 client 的 runID（老模组未发 hello 时为空串）。
func (s *Service) ClientRunIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ordered := make([]*activeClient, 0, len(s.clients))
	for _, c := range s.clients {
		ordered = append(ordered, c)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].id < ordered[j].id })
	out := make([]string, len(ordered))
	for i, c := range ordered {
		out[i] = c.runID
	}
	return out
}

// SendPacket 向最新连入 client fire-and-forget 发送一帧（单开语义）。无 client → MCDK_BRIDGE_NO_CLIENT；
// 写失败 → 从注册表摘除该 client 并返回 MCDK_BRIDGE_WRITE_FAILED。
func (s *Service) SendPacket(p Packet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.newestLocked()
	if c == nil {
		return coded(CodeBridgeNoClient, "no active mcdk client connected")
	}
	return s.writeLocked(c, p)
}

// SendPacketToRun 向 runID 匹配的最新 client fire-and-forget 发送一帧。无匹配 → MCDK_BRIDGE_NO_CLIENT；
// 写失败 → 从注册表摘除该 client 并返回 MCDK_BRIDGE_WRITE_FAILED。
func (s *Service) SendPacketToRun(runID string, p Packet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var target *activeClient
	for _, c := range s.clients {
		if c.runID == runID && (target == nil || c.id > target.id) {
			target = c
		}
	}
	if target == nil {
		return coded(CodeBridgeNoClient, fmt.Sprintf("no mcdk client connected for run %s", runID))
	}
	return s.writeLocked(target, p)
}

// writeLocked 实际发帧；调用方须已持 s.mu。写失败时从 clients map 摘除并关连接。
func (s *Service) writeLocked(c *activeClient, p Packet) error {
	if _, err := p.WriteTo(c.conn); err != nil {
		s.log.Warn("mcdk bridge write failed, dropping client", "err", err, "runId", c.runID)
		delete(s.clients, c.id)
		_ = c.conn.Close()
		return coded(CodeBridgeWriteFailed, fmt.Sprintf("write to mcdk client failed: %v", err))
	}
	return nil
}

func (s *Service) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return // listener 已关（Stop），正常退出
			}
			s.log.Warn("mcdk bridge accept failed", "err", err)
			time.Sleep(5 * time.Millisecond) // 避免持续错误时 tight loop（对齐 cc-pane yield_now）
			continue
		}

		s.mu.Lock()
		id := s.nextID
		s.nextID++
		s.clients[id] = &activeClient{
			id:            id,
			conn:          conn,
			connectedAtMs: time.Now().UnixMilli(),
			remoteAddr:    conn.RemoteAddr().String(),
		}
		s.mu.Unlock()

		go s.readDrain(conn, id)
	}
}

// readDrain 读 socket 直到 EOF / error，然后把本连接从 clients 摘除。累积入站字节、按帧解码：
// PacketHello 帧（首帧，payload=runId）写回 s.clients[myID].runID 供归属查询；PacketLogLine 帧
// （游戏内桥模组反向 tee 的 stdout 日志）用本地已知 runID 交给 logHandler；其余入站帧丢弃。
func (s *Service) readDrain(conn net.Conn, myID uint64) {
	var runID string
	buf := make([]byte, 4096)
	var acc []byte
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				pkt, consumed, derr := Decode(acc)
				if derr != nil {
					break // 帧不完整，等更多字节再解
				}
				acc = acc[consumed:]
				switch pkt.Type {
				case PacketHello:
					runID = string(pkt.Payload)
					s.mu.Lock()
					if c := s.clients[myID]; c != nil {
						c.runID = runID
					}
					s.mu.Unlock()
				case PacketLogLine:
					s.logHandlerMu.RLock()
					h := s.logHandler
					s.logHandlerMu.RUnlock()
					if h != nil {
						h(runID, string(pkt.Payload))
					}
				}
			}
			if len(acc) == 0 {
				acc = nil // 整批耗尽，释放底层数组，避免长会话累积偏移
			}
		}
		if err != nil {
			break // EOF / 错误
		}
	}
	_ = conn.Close()

	s.mu.Lock()
	delete(s.clients, myID)
	s.mu.Unlock()
}
