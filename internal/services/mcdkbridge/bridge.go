package mcdkbridge

// bridge.go — TCP server，等游戏内 Python 桥模组反向连入。忠实移植 bridge.rs。
//
// 工作模式（参考游戏内 IPCSystem.py）：
//   - Mod 是纯接收方：只读不写，不发任何握手或响应。
//   - 我们 bind 127.0.0.1:0 拿 ephemeral port，accept 一个 client。
//   - 单 active client：新连接顶替旧连接（关掉旧 TCP）。
//   - 后台 read goroutine 仅用于检测 EOF（mod 退出 → 清状态）。
//
// 隐藏坑（务必 bind 127.0.0.1，不要 ::1 / 0.0.0.0）：游戏内 Python connect(("localhost", port))，
// 若 listener 落到 IPv6（::1）而 localhost 优先解析到它之外的地址会连不上。cc-pane 与此一致。

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
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
}

// Service 是 MCDK 调试桥 TCP server。
type Service struct {
	mu         sync.Mutex
	listener   net.Listener
	active     *activeClient
	nextID     uint64
	listenPort int
	log        *slog.Logger

	logHandlerMu sync.RWMutex
	logHandler   func(line string) // 游戏内桥模组反向日志帧（PacketLogLine）的接收回调，可 nil
}

// SetLogHandler 注入反向日志帧回调（mcpe.Service 装配时调）。每收到一帧 PacketLogLine
// 就以 payload 字符串回调一次。回调在 readDrain goroutine 上同步执行，实现方应自己不阻塞。
func (s *Service) SetLogHandler(fn func(line string)) {
	s.logHandlerMu.Lock()
	s.logHandler = fn
	s.logHandlerMu.Unlock()
}

// New 构造未启动的桥（端口在 Start 后才确定）。
func New() *Service {
	return &Service{nextID: 1, log: logx.For("mcdk-bridge")}
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

// Stop 关闭 listener 与当前 client（XK 生命周期约定；cc-pane 无此方法，桥随 app 常驻）。
func (s *Service) Stop() error {
	s.mu.Lock()
	ln := s.listener
	old := s.active
	s.listener = nil
	s.active = nil
	s.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	if old != nil {
		_ = old.conn.Close()
	}
	return nil
}

// Port 返回监听端口（未 Start 时为 0）。
func (s *Service) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listenPort
}

// Status 返回当前 client 连接快照。
func (s *Service) Status() ClientStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return ClientStatus{Connected: true, RemoteAddr: s.active.remoteAddr, ConnectedAtMs: s.active.connectedAtMs}
	}
	return ClientStatus{}
}

// SendPacket 向 active client fire-and-forget 发送一帧。无 client → MCDK_BRIDGE_NO_CLIENT；
// 写失败 → 清状态并返回 MCDK_BRIDGE_WRITE_FAILED。
func (s *Service) SendPacket(p Packet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return coded(CodeBridgeNoClient, "no active mcdk client connected")
	}
	if _, err := p.WriteTo(s.active.conn); err != nil {
		// 写失败说明 socket 已不可用，清状态再返回错误。关 conn → 其 read goroutine 收到错误退出。
		s.log.Warn("mcdk bridge write failed, clearing active client", "err", err)
		old := s.active
		s.active = nil
		_ = old.conn.Close()
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
		old := s.active // 顶替旧 client
		s.active = &activeClient{
			id:            id,
			conn:          conn,
			connectedAtMs: time.Now().UnixMilli(),
			remoteAddr:    conn.RemoteAddr().String(),
		}
		s.mu.Unlock()

		if old != nil {
			// 关旧 TCP → 其 read goroutine 收到错误退出；因 active.id 已是新值，不会误清新 client。
			_ = old.conn.Close()
		}
		go s.readDrain(conn, id)
	}
}

// readDrain 读 socket 直到 EOF / error / 被顶替（conn 被关），然后（若仍是 active）清状态。
// 累积入站字节、按帧解码：PacketLogLine 帧（游戏内桥模组反向 tee 的 stdout 日志）交给
// logHandler；其余入站帧丢弃。读到 EOF 仍负责清 active。
func (s *Service) readDrain(conn net.Conn, myID uint64) {
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
				if pkt.Type == PacketLogLine {
					s.logHandlerMu.RLock()
					h := s.logHandler
					s.logHandlerMu.RUnlock()
					if h != nil {
						h(string(pkt.Payload))
					}
				}
			}
			if len(acc) == 0 {
				acc = nil // 整批耗尽，释放底层数组，避免长会话累积偏移
			}
		}
		if err != nil {
			break // EOF / 被顶替关闭 / 错误
		}
	}
	_ = conn.Close()

	s.mu.Lock()
	if s.active != nil && s.active.id == myID {
		s.active = nil
	}
	s.mu.Unlock()
}
