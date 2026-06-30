package botbridge

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"xkpane/internal/events"
	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
)

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

func TestSetConfigEncryptsAppSecretRoundTrip(t *testing.T) {
	holder := testHolder(t)
	s := New(holder)
	plain := "app-secret-for-test"

	err := s.SetConfig(context.Background(), ConfigInput{
		Platform:  "feishu",
		AppID:     "cli_a",
		AppSecret: plain,
		Enabled:   false,
	})
	if err != nil {
		t.Fatalf("SetConfig err=%v", err)
	}

	var row storage.BotBridgeConfig
	if err := holder.Current().GORM.First(&row, "id = ?", 1).Error; err != nil {
		t.Fatalf("load bot_bridge_config: %v", err)
	}
	if row.TokenEnc == "" {
		t.Fatalf("TokenEnc is empty")
	}
	if row.TokenEnc == plain {
		t.Fatalf("TokenEnc stored plaintext")
	}
	got, err := cryptox.DecryptString(row.TokenEnc)
	if err != nil {
		t.Fatalf("DecryptString err=%v", err)
	}
	if got != plain {
		t.Fatalf("decrypted secret = %q, want %q", got, plain)
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
	cards []*Card
}

func (f *fakePlatform) Name() string { return "fake" }

func (f *fakePlatform) Start(MessageHandler) error { return nil }

func (f *fakePlatform) Reply(context.Context, any, string) error { return nil }

func (f *fakePlatform) Send(context.Context, any, string) error { return nil }

func (f *fakePlatform) Stop() error { return nil }

func (f *fakePlatform) SendCard(_ context.Context, _ any, card *Card) error {
	f.cards = append(f.cards, card)
	return nil
}

func (f *fakePlatform) ReplyCard(context.Context, any, *Card) error { return nil }
