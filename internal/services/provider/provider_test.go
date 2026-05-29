package provider

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(storage.NewHolder(db))
}

func mustAdd(t *testing.T, svc *Service, in ProviderInput) ProviderDTO {
	t.Helper()
	dto, err := svc.Add(context.Background(), in)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	return dto
}

// ---- default-provider 不变式 ----

func TestAddFirstForcesDefault(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "anthropic", IsDefault: false})
	if !dto.IsDefault {
		t.Error("first provider must be forced default even when input isDefault=false")
	}
}

func TestAddSecondDefaultClearsFirst(t *testing.T) {
	svc := newTestService(t)
	a := mustAdd(t, svc, ProviderInput{Name: "a"})
	b := mustAdd(t, svc, ProviderInput{Name: "b", IsDefault: true})
	if !b.IsDefault {
		t.Error("b should be default")
	}
	ga, _ := svc.Get(context.Background(), a.ID)
	if ga == nil || ga.IsDefault {
		t.Errorf("a should no longer be default: %+v", ga)
	}
}

// ---- cryptox 加密落库 ----

func TestAddEncryptsApiKeyAtRest(t *testing.T) {
	svc := newTestService(t)
	const secret = "sk-supersecret-12345"
	dto := mustAdd(t, svc, ProviderInput{Name: "p", ProviderType: "anthropic", APIKey: secret})
	if !dto.HasAPIKey {
		t.Error("HasAPIKey should be true")
	}
	var raw string
	if err := svc.db().Raw("SELECT api_key FROM providers WHERE id = ?", dto.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if raw == "" {
		t.Fatal("api_key should be ciphertext, not empty")
	}
	if raw == secret || strings.Contains(raw, secret) {
		t.Fatalf("api_key stored in PLAINTEXT! raw=%q", raw)
	}
	dec, err := cryptox.DecryptString(raw)
	if err != nil || dec != secret {
		t.Fatalf("decrypt round-trip: err=%v got=%q want=%q", err, dec, secret)
	}
}

func TestAddNoKeyHasApiKeyFalse(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "bedrock", ProviderType: "bedrock", AWSProfile: "prod"})
	if dto.HasAPIKey {
		t.Error("provider with no apiKey should have HasAPIKey=false")
	}
}

// ---- Get ----

func TestGetNotFoundReturnsNil(t *testing.T) {
	svc := newTestService(t)
	p, err := svc.Get(context.Background(), "nope")
	if err != nil {
		t.Fatalf("Get err = %v, want nil", err)
	}
	if p != nil {
		t.Fatalf("Get(missing) = %v, want nil", p)
	}
}

// ---- Update: patch 保留 key / 不动 default ----

func TestUpdatePreservesKeyOnEmpty(t *testing.T) {
	svc := newTestService(t)
	const secret = "sk-keep-me"
	a := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "anthropic", APIKey: secret})
	// 空 apiKey + 改名 → key 应保留
	if err := svc.Update(context.Background(), a.ID, ProviderInput{Name: "renamed", ProviderType: "anthropic", APIKey: ""}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := svc.Get(context.Background(), a.ID)
	if got == nil || got.Name != "renamed" {
		t.Fatalf("name not updated: %+v", got)
	}
	if !got.HasAPIKey {
		t.Fatal("apiKey should be preserved on empty-input update")
	}
	var raw string
	svc.db().Raw("SELECT api_key FROM providers WHERE id = ?", a.ID).Scan(&raw)
	dec, _ := cryptox.DecryptString(raw)
	if dec != secret {
		t.Fatalf("preserved key = %q, want %q", dec, secret)
	}
}

func TestUpdateReplacesKeyWhenProvided(t *testing.T) {
	svc := newTestService(t)
	a := mustAdd(t, svc, ProviderInput{Name: "a", APIKey: "old"})
	if err := svc.Update(context.Background(), a.ID, ProviderInput{Name: "a", APIKey: "new-key"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	var raw string
	svc.db().Raw("SELECT api_key FROM providers WHERE id = ?", a.ID).Scan(&raw)
	dec, _ := cryptox.DecryptString(raw)
	if dec != "new-key" {
		t.Fatalf("key = %q, want new-key", dec)
	}
}

func TestUpdateDoesNotTouchDefault(t *testing.T) {
	svc := newTestService(t)
	a := mustAdd(t, svc, ProviderInput{Name: "a"}) // forced default
	// Update 传 isDefault=false，不应清掉 a 的默认（Update 不管 default）
	if err := svc.Update(context.Background(), a.ID, ProviderInput{Name: "a2", IsDefault: false}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := svc.Get(context.Background(), a.ID)
	if got == nil || !got.IsDefault {
		t.Fatalf("default flag must be untouched by Update: %+v", got)
	}
}

func TestUpdateNotFound(t *testing.T) {
	svc := newTestService(t)
	if err := svc.Update(context.Background(), "nope", ProviderInput{Name: "x"}); !errors.Is(err, errNotFound) {
		t.Fatalf("Update(missing) = %v, want errNotFound", err)
	}
}

// ---- Remove + 默认提升 ----

func TestRemovePromotesOldestRemaining(t *testing.T) {
	svc := newTestService(t)
	a := mustAdd(t, svc, ProviderInput{Name: "a"}) // default
	b := mustAdd(t, svc, ProviderInput{Name: "b"})
	_ = mustAdd(t, svc, ProviderInput{Name: "c"})
	if err := svc.Remove(context.Background(), a.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	gb, _ := svc.Get(context.Background(), b.ID)
	if gb == nil || !gb.IsDefault {
		t.Fatalf("oldest remaining (b) should be promoted to default: %+v", gb)
	}
}

func TestRemoveNonDefaultKeepsDefault(t *testing.T) {
	svc := newTestService(t)
	a := mustAdd(t, svc, ProviderInput{Name: "a"}) // default
	b := mustAdd(t, svc, ProviderInput{Name: "b"})
	if err := svc.Remove(context.Background(), b.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	ga, _ := svc.Get(context.Background(), a.ID)
	if ga == nil || !ga.IsDefault {
		t.Fatalf("a should remain default: %+v", ga)
	}
}

func TestRemoveNotFound(t *testing.T) {
	svc := newTestService(t)
	if err := svc.Remove(context.Background(), "nope"); !errors.Is(err, errNotFound) {
		t.Fatalf("Remove(missing) = %v, want errNotFound", err)
	}
}

// ---- SetDefault ----

func TestSetDefault(t *testing.T) {
	svc := newTestService(t)
	a := mustAdd(t, svc, ProviderInput{Name: "a"}) // default
	c := mustAdd(t, svc, ProviderInput{Name: "c"})
	if err := svc.SetDefault(context.Background(), c.ID); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	ga, _ := svc.Get(context.Background(), a.ID)
	gc, _ := svc.Get(context.Background(), c.ID)
	if ga == nil || ga.IsDefault {
		t.Errorf("a should not be default")
	}
	if gc == nil || !gc.IsDefault {
		t.Errorf("c should be default")
	}
}

func TestSetDefaultUnknownId(t *testing.T) {
	svc := newTestService(t)
	mustAdd(t, svc, ProviderInput{Name: "a"})
	if err := svc.SetDefault(context.Background(), "nope"); !errors.Is(err, errNotFound) {
		t.Fatalf("SetDefault(unknown) = %v, want errNotFound", err)
	}
}

// ---- List ----

func TestListEmptyNonNil(t *testing.T) {
	svc := newTestService(t)
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list == nil {
		t.Fatal("List should return non-nil empty slice")
	}
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}
}

func TestListMasksKeyAndOrders(t *testing.T) {
	svc := newTestService(t)
	mustAdd(t, svc, ProviderInput{Name: "a", APIKey: "secret-a"})
	mustAdd(t, svc, ProviderInput{Name: "b"})
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	// 插入序（created_at 升序）
	if list[0].Name != "a" || list[1].Name != "b" {
		t.Fatalf("order = %s,%s want a,b", list[0].Name, list[1].Name)
	}
	// 脱敏：DTO 类型本身不含明文字段；hasApiKey 正确反映
	if !list[0].HasAPIKey {
		t.Error("a should report HasAPIKey=true")
	}
	if list[1].HasAPIKey {
		t.Error("b should report HasAPIKey=false")
	}
}
