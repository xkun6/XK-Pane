package botbridge

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/services/terminal"
	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
)

// registerFakeFeishu 确保测试进程内 "feishu" 平台有一个工厂——SetConfig 现在会校验平台
// 是否已注册适配器，而 botbridge 包测试不会 blank import platform/feishu。
func registerFakeFeishu(t *testing.T) {
	t.Helper()
	if _, ok := platformFactories["feishu"]; ok {
		return
	}
	RegisterPlatform("feishu", func(map[string]any) (Platform, error) {
		return &fakePlatform{}, nil
	})
}

func TestSetConfigRejectsUnsupportedPlatform(t *testing.T) {
	s := New(testHolder(t))
	err := s.SetConfig(context.Background(), ConfigInput{Platform: "telegram"})
	if err == nil {
		t.Fatalf("SetConfig err=nil, want unsupported platform error")
	}
}

func TestGetStatusReturnsDefaultWhenConfigMissing(t *testing.T) {
	s := New(testHolder(t))
	st, err := s.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus err=%v", err)
	}
	if st.Platform != "feishu" || st.Enabled || st.Connected {
		t.Fatalf("status = %#v, want default feishu disabled disconnected", st)
	}
}

func TestSetConfigEncryptsCredentialsRoundTrip(t *testing.T) {
	holder := testHolder(t)
	s := New(holder)
	registerFakeFeishu(t)
	plain := "app-secret-for-test"

	err := s.SetConfig(context.Background(), ConfigInput{
		Platform:    "feishu",
		Credentials: map[string]string{"app_id": "cli_a", "app_secret": plain},
		Enabled:     false,
	})
	if err != nil {
		t.Fatalf("SetConfig err=%v", err)
	}

	var row storage.BotBridgeConfig
	if err := holder.Current().GORM.First(&row, "id = ?", 1).Error; err != nil {
		t.Fatalf("load bot_bridge_config: %v", err)
	}
	if row.CredentialsEnc == "" {
		t.Fatalf("CredentialsEnc is empty")
	}
	if strings.Contains(row.CredentialsEnc, plain) {
		t.Fatalf("CredentialsEnc 含明文密钥")
	}
	dec, err := cryptox.DecryptString(row.CredentialsEnc)
	if err != nil {
		t.Fatalf("DecryptString err=%v", err)
	}
	var creds map[string]string
	if err := json.Unmarshal([]byte(dec), &creds); err != nil {
		t.Fatalf("凭据 JSON 解析失败: %v", err)
	}
	if creds["app_secret"] != plain {
		t.Fatalf("decrypted app_secret = %q, want %q", creds["app_secret"], plain)
	}
	if creds["app_id"] != "cli_a" {
		t.Fatalf("app_id = %q, want cli_a", creds["app_id"])
	}

	// 非敏感字段经 GetStatus 回传、敏感字段不回传。
	st, err := s.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus err=%v", err)
	}
	if st.Credentials["app_id"] != "cli_a" {
		t.Fatalf("status app_id = %q, want cli_a", st.Credentials["app_id"])
	}
	if _, ok := st.Credentials["app_secret"]; ok {
		t.Fatalf("status 不应回传敏感字段 app_secret")
	}
}

func TestPushAskCardOnlyPushesWhenToolUseChanges(t *testing.T) {
	p := &fakePlatform{}
	s := New(testHolder(t))
	s.platformMu.Lock()
	s.platform = p
	s.connected = true
	s.platformMu.Unlock()
	s.dispatch.setBinding("scope-a", "sid-1")

	s.pushAskCard("sid-1", PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "Pick", Options: []events.AskOption{{Label: "A"}}},
		},
	})
	s.pushAskCard("sid-1", PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "Pick", Options: []events.AskOption{{Label: "A"}}},
		},
	})
	s.pushAskCard("sid-1", PendingAsk{
		ToolUseID: "tool-2",
		Questions: []events.AskQuestion{
			{Question: "Again", Options: []events.AskOption{{Label: "B"}}},
		},
	})

	if got, want := len(p.cards), 2; got != want {
		t.Fatalf("pushed cards = %d, want %d", got, want)
	}
}

func TestPushAskCardRetriesAfterSendFailure(t *testing.T) {
	p := &fakePlatform{sendErr: errors.New("platform offline")}
	s := New(testHolder(t))
	s.platformMu.Lock()
	s.platform = p
	s.connected = true
	s.platformMu.Unlock()
	s.dispatch.setBinding("scope-a", "sid-1")

	ask := PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "Pick", Options: []events.AskOption{{Label: "A"}}},
		},
	}

	// 首推：平台离线 → 全部发送失败，去重占位应被回滚，没有任何卡投递。
	s.pushAskCard("sid-1", ask)
	if len(p.cards) != 0 {
		t.Fatalf("after failed push, delivered cards = %d, want 0", len(p.cards))
	}

	// 平台恢复后下一轮重推：dedup 已清，应真正投递（验证「失败清 dedup 重推」）。
	p.sendErr = nil
	s.pushAskCard("sid-1", ask)
	if len(p.cards) == 0 {
		t.Fatalf("after recovery, expected re-push to deliver cards, got 0")
	}

	// 同一 toolUseID 再推：dedup 生效，不应重复发送。
	before := len(p.cards)
	s.pushAskCard("sid-1", ask)
	if len(p.cards) != before {
		t.Fatalf("duplicate push delivered extra cards: before=%d after=%d", before, len(p.cards))
	}
}

func TestConnectionStatusCallbackDrivesStatus(t *testing.T) {
	s := New(testHolder(t))

	s.handleConnectionStatus(true, nil)
	st, err := s.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus err=%v", err)
	}
	if !st.Connected || st.Error != "" {
		t.Fatalf("status after connected = %#v", st)
	}

	s.handleConnectionStatus(false, errors.New("bad credentials"))
	st, err = s.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus err=%v", err)
	}
	if st.Connected || st.Error != "bad credentials" {
		t.Fatalf("status after error = %#v", st)
	}
}

func TestStartFailureStillRunsPollLoop(t *testing.T) {
	// 回归：Start 时平台创建失败（凭据错误等）不得连带跳过 pollLoop——
	// 否则 s.ctx 已赋值使二次 Start no-op、SetConfig 只 reload 平台，
	// 事后修好凭据也永远没有镜像/ask 卡推送。
	holder := testHolder(t)
	const name = "failing-start"
	if _, ok := platformFactories[name]; !ok {
		RegisterPlatform(name, func(map[string]any) (Platform, error) {
			return nil, errors.New("bad credentials")
		})
	}
	if err := holder.Current().GORM.Save(&storage.BotBridgeConfig{ID: 1, Platform: name, Enabled: true, BindingsJSON: "{}"}).Error; err != nil {
		t.Fatalf("seed config: %v", err)
	}
	s := New(holder)
	s.AttachServices(BotDeps{Sessions: &fakeLister{snaps: []terminal.SessionSnapshot{{ID: "sid-1"}}}})
	if err := s.Start(); err == nil {
		t.Fatalf("Start err=nil, want platform create failure")
	}
	t.Cleanup(func() { _ = s.Stop() })

	deadline := time.Now().Add(5 * time.Second)
	for {
		s.pollMu.Lock()
		_, ok := s.pollers["sid-1"]
		s.pollMu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("pollLoop 未在 Start 失败后运行（sid-1 无 poller）")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

type fakeLister struct{ snaps []terminal.SessionSnapshot }

func (f *fakeLister) SessionSnapshots() []terminal.SessionSnapshot { return f.snaps }

func testHolder(t *testing.T) *storage.Holder {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "xkpane-test.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return storage.NewHolder(db)
}

type fakePlatform struct {
	cards   []*Card
	sendErr error
}

func (f *fakePlatform) Name() string { return "fake" }

func (f *fakePlatform) Start(MessageHandler) error { return nil }

func (f *fakePlatform) Reply(context.Context, any, string) error { return nil }

func (f *fakePlatform) Send(context.Context, any, string) error { return nil }

func (f *fakePlatform) Stop() error { return nil }

func (f *fakePlatform) SendCard(_ context.Context, _ any, card *Card) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.cards = append(f.cards, card)
	return nil
}

func (f *fakePlatform) ReplyCard(context.Context, any, *Card) error { return nil }
