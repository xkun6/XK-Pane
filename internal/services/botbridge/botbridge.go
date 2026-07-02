package botbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/services/orchestrator"
	"xkpane/internal/services/terminal"
	"xkpane/internal/services/transcript"
	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
	"xkpane/internal/utils/logx"

	"gorm.io/gorm"
)

const pollInterval = 1500 * time.Millisecond

type SessionLister interface {
	SessionSnapshots() []terminal.SessionSnapshot
}

type AskPender interface {
	PendingAsk(sessionID string) (orchestrator.PendingAsk, bool)
}

type BotDeps struct {
	Sessions SessionLister
	Writer   sessionWriter
	AskPend  AskPender
	Scanner  sessionScanner
}

type Status struct {
	Enabled     bool              `json:"enabled"`
	Platform    string            `json:"platform"`
	Credentials map[string]string `json:"credentials"` // 仅非敏感字段，敏感字段（密钥）不回传
	Connected   bool              `json:"connected"`
	Error       string            `json:"error,omitempty"`
}

type ConfigInput struct {
	Platform    string            `json:"platform"`
	Credentials map[string]string `json:"credentials"` // 各平台凭据字段，键名见 platformSpecs
	Enabled     bool              `json:"enabled"`
}

type Service struct {
	holder *storage.Holder
	log    *logxLogger

	depsMu sync.RWMutex
	deps   BotDeps

	ctxMu  sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	platformMu sync.Mutex
	platform   Platform
	connected  bool
	lastErr    string

	dispatch *dispatcher

	writeLocksMu sync.Mutex
	writeLocks   map[string]*sync.Mutex

	pollMu    sync.Mutex
	pollers   map[string]context.CancelFunc
	pushedAsk map[string]string
}

type logxLogger struct{}

func (logxLogger) Info(msg string, args ...any)  { logx.For("botbridge").Info(msg, args...) }
func (logxLogger) Warn(msg string, args ...any)  { logx.For("botbridge").Warn(msg, args...) }
func (logxLogger) Error(msg string, args ...any) { logx.For("botbridge").Error(msg, args...) }

func New(holder *storage.Holder) *Service {
	s := &Service{
		holder:     holder,
		log:        &logxLogger{},
		writeLocks: map[string]*sync.Mutex{},
		pollers:    map[string]context.CancelFunc{},
		pushedAsk:  map[string]string{},
	}
	s.dispatch = newDispatcher(dispatchDeps{
		writer:           nil,
		ask:              s.pendingAsk,
		listSessions:     s.sessionSnapshots,
		saveBinding:      s.saveBinding,
		sendText:         s.sendTextToScope,
		sendCard:         s.sendCardToScope,
		sessionWriteLock: s.sessionWriteLock,
		askAnswered:      s.clearPushedAsk,
	})
	return s
}

func (s *Service) AttachServices(deps BotDeps) {
	s.depsMu.Lock()
	s.deps = deps
	s.depsMu.Unlock()
	s.dispatch.deps.writer = deps.Writer
}

func (s *Service) Start() error {
	s.ctxMu.Lock()
	if s.ctx != nil {
		s.ctxMu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel
	s.ctxMu.Unlock()

	if err := s.loadBindings(); err != nil {
		s.setErr(err)
	}
	// pollLoop 必须先于 reloadPlatform 无条件启动：一旦先 reload 且失败即 return，
	// s.ctx 已赋值使二次 Start 变 no-op、SetConfig 又只 reload 平台不补启轮询，
	// 用户事后在设置页修好凭据会得到「已连接」但镜像/ask 卡永不推送的假活状态。
	go s.pollLoop(ctx)
	if err := s.reloadPlatform(ctx); err != nil {
		s.setErr(err)
		return err
	}
	return nil
}

func (s *Service) Stop() error {
	s.ctxMu.Lock()
	cancel := s.cancel
	s.ctx = nil
	s.cancel = nil
	s.ctxMu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.stopPlatform()
	s.pollMu.Lock()
	for id, c := range s.pollers {
		c()
		delete(s.pollers, id)
	}
	s.pollMu.Unlock()
	return nil
}

func (s *Service) RouteEvent(name string, payload any) {
	switch name {
	case events.TerminalAskPending:
		p, ok := payload.(events.TerminalAskPendingPayload)
		if !ok {
			return
		}
		go func() {
			pending, ok := s.pendingAsk(p.SessionID)
			if !ok {
				s.log.Warn("收到 ask-pending 事件，但 orchestrator 未返回待答（可能 sessionId 不匹配或待答已清除）", "session", p.SessionID)
				return
			}
			s.log.Info("收到 ask-pending 事件，登记待答（卡片由轮询器统一推送，保证与镜像卡顺序一致）", "session", p.SessionID, "toolUseId", pending.ToolUseID, "questions", len(pending.Questions))
			s.dispatch.setPending(p.SessionID, pending)
		}()
	case events.TerminalExit:
		if p, ok := payload.(events.TerminalExitPayload); ok {
			s.stopPoller(p.SessionID)
			s.clearPushedAsk(p.SessionID, "")
		}
	}
}

// ListPlatforms 返回已注册适配器的平台凭据规格，供前端配置页动态渲染输入框。
func (s *Service) ListPlatforms() []PlatformSpec {
	return registeredPlatformSpecs()
}

func (s *Service) GetStatus() (Status, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return Status{}, err
	}
	public := map[string]string{}
	for k, v := range s.decodeCredentials(cfg) {
		if !isSecretField(cfg.Platform, k) {
			public[k] = v
		}
	}
	s.platformMu.Lock()
	connected := s.connected
	lastErr := s.lastErr
	s.platformMu.Unlock()
	return Status{Enabled: cfg.Enabled, Platform: cfg.Platform, Credentials: public, Connected: connected, Error: lastErr}, nil
}

func (s *Service) SetConfig(ctx context.Context, in ConfigInput) error {
	platform := strings.TrimSpace(in.Platform)
	if platform == "" {
		platform = "feishu"
	}
	if _, ok := platformFactories[platform]; !ok {
		return fmt.Errorf("unsupported bot platform %q", platform)
	}
	old, _ := s.loadConfig()
	oldCreds := s.decodeCredentials(old)
	samePlatform := old.Platform == platform
	sp, _ := platformSpec(platform)
	creds := map[string]string{}
	for _, f := range sp.Fields {
		v := strings.TrimSpace(in.Credentials[f.Key])
		// 敏感字段留空 + 同平台 → 沿用旧值（前端保存时不回显密钥，空=不变）。
		if v == "" && f.Secret && samePlatform {
			v = oldCreds[f.Key]
		}
		if v != "" {
			creds[f.Key] = v
		}
	}
	credsEnc, err := encodeCredentials(creds)
	if err != nil {
		return err
	}
	row := storage.BotBridgeConfig{
		ID:             1,
		Platform:       platform,
		CredentialsEnc: credsEnc,
		Enabled:        in.Enabled,
		BindingsJSON:   "{}",
		Extra:          compactJSON(map[string]any{"mode": "websocket"}),
	}
	if old.BindingsJSON != "" {
		row.BindingsJSON = old.BindingsJSON
	}
	if err := s.holder.Current().GORM.WithContext(ctx).Save(&row).Error; err != nil {
		return err
	}
	s.ctxMu.Lock()
	runCtx := s.ctx
	s.ctxMu.Unlock()
	if runCtx != nil {
		if err := s.reloadPlatform(runCtx); err != nil {
			s.setErr(err)
			return err
		}
	}
	return nil
}

// encodeCredentials 把凭据 map 序列化并整体加密后存 CredentialsEnc。
func encodeCredentials(creds map[string]string) (string, error) {
	b, err := json.Marshal(creds)
	if err != nil {
		return "", err
	}
	return cryptox.EncryptString(string(b))
}

// decodeCredentials 解出凭据 map：优先读新版 CredentialsEnc；为空则向后兼容旧版飞书
// 单平台的 AppID + TokenEnc 两列，合成出 {app_id, app_secret}。
func (s *Service) decodeCredentials(row storage.BotBridgeConfig) map[string]string {
	if strings.TrimSpace(row.CredentialsEnc) != "" {
		if dec, err := cryptox.DecryptString(row.CredentialsEnc); err == nil {
			var m map[string]string
			if json.Unmarshal([]byte(dec), &m) == nil && m != nil {
				return m
			}
		}
	}
	out := map[string]string{}
	if strings.TrimSpace(row.AppID) != "" {
		out["app_id"] = row.AppID
	}
	if strings.TrimSpace(row.TokenEnc) != "" {
		if sec, err := cryptox.DecryptString(row.TokenEnc); err == nil {
			out["app_secret"] = sec
		}
	}
	return out
}

func (s *Service) loadConfig() (storage.BotBridgeConfig, error) {
	var row storage.BotBridgeConfig
	err := s.holder.Current().GORM.First(&row, "id = ?", 1).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return storage.BotBridgeConfig{ID: 1, Platform: "feishu", BindingsJSON: "{}"}, nil
		}
		return storage.BotBridgeConfig{ID: 1, Platform: "feishu", BindingsJSON: "{}"}, err
	}
	if row.Platform == "" {
		row.Platform = "feishu"
	}
	return row, nil
}

func (s *Service) reloadPlatform(ctx context.Context) error {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil
	}
	s.stopPlatform()
	if !cfg.Enabled {
		s.platformMu.Lock()
		s.connected = false
		s.lastErr = ""
		s.platformMu.Unlock()
		return nil
	}
	opts := map[string]any{}
	for k, v := range s.decodeCredentials(cfg) {
		opts[k] = v
	}
	p, err := CreatePlatform(cfg.Platform, opts)
	if err != nil {
		return err
	}
	if reporter, ok := p.(ConnectionReporter); ok {
		reporter.SetConnectionHandler(s.handleConnectionStatus)
	}
	if err := p.Start(s.handlePlatformMessage); err != nil {
		return err
	}
	s.platformMu.Lock()
	s.platform = p
	s.lastErr = ""
	s.platformMu.Unlock()
	_ = ctx
	return nil
}

func (s *Service) stopPlatform() {
	s.platformMu.Lock()
	p := s.platform
	s.platform = nil
	s.connected = false
	s.platformMu.Unlock()
	if p != nil {
		_ = p.Stop()
	}
}

func (s *Service) handlePlatformMessage(p Platform, msg *Message) {
	if msg == nil {
		return
	}
	ctx := context.Background()
	if handled, err := s.dispatch.handleAction(ctx, msg.ScopeKey, msg.MessageID, msg.Content); err != nil {
		s.log.Warn("handle platform action failed", "err", err)
	} else if handled {
		return
	}
	if _, err := s.dispatch.handleMessage(ctx, msg.ScopeKey, msg.MessageID, msg.Content); err != nil {
		s.log.Warn("handle platform message failed", "err", err)
	}
}

func (s *Service) handleConnectionStatus(connected bool, err error) {
	s.platformMu.Lock()
	s.connected = connected
	if err != nil {
		s.lastErr = err.Error()
	} else if connected {
		s.lastErr = ""
	}
	s.platformMu.Unlock()
}

func (s *Service) pollLoop(ctx context.Context) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, snap := range s.sessionSnapshots() {
				s.ensurePoller(ctx, snap)
			}
		}
	}
}

func (s *Service) ensurePoller(ctx context.Context, snap sessionSnapshot) {
	s.pollMu.Lock()
	if _, ok := s.pollers[snap.ID]; ok {
		s.pollMu.Unlock()
		return
	}
	pctx, cancel := context.WithCancel(ctx)
	s.pollers[snap.ID] = cancel
	s.pollMu.Unlock()
	go s.pollSession(pctx, snap)
}

func (s *Service) stopPoller(sessionID string) {
	s.pollMu.Lock()
	if cancel := s.pollers[sessionID]; cancel != nil {
		cancel()
		delete(s.pollers, sessionID)
	}
	s.pollMu.Unlock()
}

func (s *Service) pollSession(ctx context.Context, snap sessionSnapshot) {
	after := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
		s.depsMu.RLock()
		scanner := s.deps.Scanner
		s.depsMu.RUnlock()
		path, ok := locateTranscript(ctx, scanner, snap)
		if !ok {
			continue
		}
		res := parseTimeline(path, snap, after)
		after = res.Total
		for _, ev := range res.Events {
			// 仅在 tool_result（答案落地）时标记 seen / 清除 ask 卡去重。
			// 不能在 tool_use 出现时就标记：reclaude 待答期会先把 AskUserQuestion 的 tool_use
			// 写进转录，若此时标记 seen 会令 effectivePending 误判已答、提前清除待答，
			// 导致用户点「提交全部」时待答已消失、submitAsk 静默返回而无反应。
			if ev.Kind == transcript.KindToolResult && ev.ToolUseID != "" {
				s.dispatch.markToolUseSeen(snap.ID, ev.ToolUseID)
				s.clearPushedAsk(snap.ID, ev.ToolUseID)
			}
			card, ok := eventToCard(ev)
			if !ok {
				continue
			}
			for _, scope := range s.dispatch.bindingsForSession(snap.ID) {
				if err := s.sendCardToScope(ctx, scope, nil, card); err != nil {
					s.log.Warn("推送镜像卡片失败", "session", snap.ID, "scope", scope, "err", err)
				}
			}
		}
		if pending, ok := s.dispatch.effectivePending(snap.ID); ok {
			s.pushAskCard(snap.ID, pending)
		}
	}
}

func (s *Service) pushAskCard(sessionID string, p PendingAsk) {
	if p.ToolUseID == "" {
		s.log.Warn("跳过 AskUserQuestion 卡片：toolUseId 为空", "session", sessionID)
		return
	}
	scopes := s.dispatch.bindingsForSession(sessionID)
	if len(scopes) == 0 {
		s.log.Warn("跳过 AskUserQuestion 卡片：该会话未在 IM 绑定，请先在飞书内用 /sessions 绑定该会话", "session", sessionID, "toolUseId", p.ToolUseID)
		return
	}
	s.pollMu.Lock()
	if s.pushedAsk[sessionID] == p.ToolUseID {
		s.pollMu.Unlock()
		return
	}
	// 先占去重位，避免后续轮询重复推；若本轮一张都没投递成功，再回滚占位让下一轮重推。
	s.pushedAsk[sessionID] = p.ToolUseID
	s.pollMu.Unlock()
	delivered := false
	if s.platformIsTextOnly() {
		// 纯文本平台无按钮：推一条编号作答提示，用户回复编号即作答（见 dispatcher 文本作答兜底）。
		text := askText(p)
		for _, scope := range scopes {
			if err := s.sendTextToScope(context.Background(), scope, nil, text); err != nil {
				s.log.Warn("推送 AskUserQuestion 文本作答提示失败", "session", sessionID, "scope", scope, "toolUseId", p.ToolUseID, "err", err)
				continue
			}
			delivered = true
			s.log.Info("已推送 AskUserQuestion 文本作答提示", "session", sessionID, "scope", scope, "toolUseId", p.ToolUseID)
		}
	} else {
		cards := askCards(p)
		for _, scope := range scopes {
			sent := 0
			for _, card := range cards {
				if err := s.sendCardToScope(context.Background(), scope, nil, card); err != nil {
					s.log.Warn("推送 AskUserQuestion 卡片失败", "session", sessionID, "scope", scope, "toolUseId", p.ToolUseID, "err", err)
					continue
				}
				sent++
			}
			if sent > 0 {
				delivered = true
			}
			s.log.Info("已推送 AskUserQuestion 卡片", "session", sessionID, "scope", scope, "toolUseId", p.ToolUseID, "cards", sent, "total", len(cards))
		}
	}
	if !delivered {
		// 全部 scope 都没投递成功（常见于平台 WS 尚未握手 / 临时断连）：清掉去重占位，
		// 让下一轮轮询自动重推，避免待答卡因一次失败被去重永久挡住。仅当占位仍指向本
		// toolUseID 时才清，防止覆盖期间已登记的更新待答。用「至少投递一张=成功」而非
		// 「全成才算成功」，避免多绑定里个别坏 scope 导致对正常 scope 每轮重发刷屏。
		s.pollMu.Lock()
		if s.pushedAsk[sessionID] == p.ToolUseID {
			delete(s.pushedAsk, sessionID)
		}
		s.pollMu.Unlock()
		s.log.Warn("AskUserQuestion 卡片全部发送失败，已清除去重占位，下一轮轮询将重推", "session", sessionID, "toolUseId", p.ToolUseID)
	}
}

func (s *Service) clearPushedAsk(sessionID, toolUseID string) {
	if sessionID == "" {
		return
	}
	s.pollMu.Lock()
	if toolUseID == "" || s.pushedAsk[sessionID] == toolUseID {
		delete(s.pushedAsk, sessionID)
	}
	s.pollMu.Unlock()
}

// platformIsTextOnly 判断当前平台是否无富卡 / 无内联按钮能力（企业微信 / QQ OneBot / 微信）。
// 与 sendCardToScope 的能力降级判定保持一致：既非 CardSender 也非 InlineButtonSender 即纯文本。
func (s *Service) platformIsTextOnly() bool {
	s.platformMu.Lock()
	p := s.platform
	s.platformMu.Unlock()
	if p == nil {
		return false
	}
	if _, ok := p.(CardSender); ok {
		return false
	}
	if _, ok := p.(InlineButtonSender); ok {
		return false
	}
	return true
}

func (s *Service) sendTextToScope(ctx context.Context, scopeKey string, replyCtx any, content string) error {
	s.platformMu.Lock()
	p := s.platform
	s.platformMu.Unlock()
	if p == nil {
		return errors.New("botbridge platform not connected")
	}
	if replyCtx == nil {
		replyCtx = scopeKey
	}
	return p.Send(ctx, replyCtx, content)
}

func (s *Service) sendCardToScope(ctx context.Context, scopeKey string, replyCtx any, card *Card) error {
	s.platformMu.Lock()
	p := s.platform
	s.platformMu.Unlock()
	if p == nil {
		return errors.New("botbridge platform not connected")
	}
	if replyCtx == nil {
		replyCtx = scopeKey
	}
	if cs, ok := p.(CardSender); ok {
		return cs.SendCard(ctx, replyCtx, card)
	}
	if ib, ok := p.(InlineButtonSender); ok {
		return ib.SendWithButtons(ctx, replyCtx, card.RenderText(), card.CollectButtons())
	}
	return p.Send(ctx, replyCtx, card.RenderText())
}

func (s *Service) pendingAsk(sessionID string) (PendingAsk, bool) {
	s.depsMu.RLock()
	askPend := s.deps.AskPend
	s.depsMu.RUnlock()
	if askPend == nil {
		return PendingAsk{}, false
	}
	p, ok := askPend.PendingAsk(sessionID)
	if !ok {
		return PendingAsk{}, false
	}
	return PendingAsk{ToolUseID: p.ToolUseID, Questions: p.Questions}, true
}

func (s *Service) sessionSnapshots() []sessionSnapshot {
	s.depsMu.RLock()
	lister := s.deps.Sessions
	s.depsMu.RUnlock()
	if lister == nil {
		return nil
	}
	raw := lister.SessionSnapshots()
	out := make([]sessionSnapshot, 0, len(raw))
	for _, snap := range raw {
		out = append(out, fromTerminalSnapshot(snap))
	}
	return out
}

func (s *Service) sessionWriteLock(sessionID string) *sync.Mutex {
	s.writeLocksMu.Lock()
	defer s.writeLocksMu.Unlock()
	if s.writeLocks[sessionID] == nil {
		s.writeLocks[sessionID] = &sync.Mutex{}
	}
	return s.writeLocks[sessionID]
}

func (s *Service) loadBindings() error {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil
	}
	var bindings map[string]string
	if strings.TrimSpace(cfg.BindingsJSON) != "" {
		if err := json.Unmarshal([]byte(cfg.BindingsJSON), &bindings); err != nil {
			return err
		}
	}
	if bindings == nil {
		bindings = map[string]string{}
	}
	s.dispatch.replaceBindings(bindings)
	return nil
}

func (s *Service) saveBinding(scopeKey, sessionID string) {
	cfg, err := s.loadConfig()
	if err != nil {
		cfg = storage.BotBridgeConfig{ID: 1, Platform: "feishu", BindingsJSON: "{}"}
	}
	var bindings map[string]string
	_ = json.Unmarshal([]byte(cfg.BindingsJSON), &bindings)
	if bindings == nil {
		bindings = map[string]string{}
	}
	bindings[scopeKey] = sessionID
	cfg.BindingsJSON = compactJSON(bindings)
	if cfg.Platform == "" {
		cfg.Platform = "feishu"
	}
	_ = s.holder.Current().GORM.Save(&cfg).Error
}

func (s *Service) setErr(err error) {
	if err == nil {
		return
	}
	s.platformMu.Lock()
	s.lastErr = err.Error()
	s.connected = false
	s.platformMu.Unlock()
}
