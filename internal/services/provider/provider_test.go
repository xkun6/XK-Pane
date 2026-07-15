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
	dto := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "gemini", IsDefault: false})
	if !dto.IsDefault {
		t.Error("first provider must be forced default even when input isDefault=false")
	}
}

func TestAddSecondDefaultClearsFirst(t *testing.T) {
	svc := newTestService(t)
	a := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "gemini"})
	b := mustAdd(t, svc, ProviderInput{Name: "b", ProviderType: "gemini", IsDefault: true})
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
	dto := mustAdd(t, svc, ProviderInput{Name: "p", ProviderType: "glm", APIKey: secret})
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
	dto := mustAdd(t, svc, ProviderInput{Name: "no-key", ProviderType: "opencode"})
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
	a := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "kimi", APIKey: secret})
	// 空 apiKey + 改名 → key 应保留
	if err := svc.Update(context.Background(), a.ID, ProviderInput{Name: "renamed", ProviderType: "kimi", APIKey: ""}); err != nil {
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
	a := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "glm", APIKey: "old"})
	if err := svc.Update(context.Background(), a.ID, ProviderInput{Name: "a", ProviderType: "glm", APIKey: "new-key"}); err != nil {
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
	a := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "gemini"}) // forced default
	// Update 传 isDefault=false，不应清掉 a 的默认（Update 不管 default）
	if err := svc.Update(context.Background(), a.ID, ProviderInput{Name: "a2", ProviderType: "gemini", IsDefault: false}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := svc.Get(context.Background(), a.ID)
	if got == nil || !got.IsDefault {
		t.Fatalf("default flag must be untouched by Update: %+v", got)
	}
}

func TestUpdateNotFound(t *testing.T) {
	svc := newTestService(t)
	if err := svc.Update(context.Background(), "nope", ProviderInput{Name: "x", ProviderType: "gemini"}); !errors.Is(err, errNotFound) {
		t.Fatalf("Update(missing) = %v, want errNotFound", err)
	}
}

// ---- Remove + 默认提升 ----

func TestRemovePromotesOldestRemaining(t *testing.T) {
	svc := newTestService(t)
	a := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "gemini"}) // default
	b := mustAdd(t, svc, ProviderInput{Name: "b", ProviderType: "gemini"})
	_ = mustAdd(t, svc, ProviderInput{Name: "c", ProviderType: "gemini"})
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
	a := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "gemini"}) // default
	b := mustAdd(t, svc, ProviderInput{Name: "b", ProviderType: "gemini"})
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
	a := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "gemini"}) // default
	c := mustAdd(t, svc, ProviderInput{Name: "c", ProviderType: "gemini"})
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
	mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "gemini"})
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
	mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "gemini", APIKey: "secret-a"})
	mustAdd(t, svc, ProviderInput{Name: "b", ProviderType: "gemini"})
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

// ---- provider 类型白名单（claude/codex 已迁移至 agentcfg）----

// anthropic/proxy/bedrock/vertex/open_ai 已一次性迁移到 agentcfg 并从本表删除；config_profile
// 无法用全局配置模型表达、永久休眠。这 6 种类型 Add 一律被拒，引导用户改用设置页的全局配置 Provider。
func TestAddRejectsLegacyClaudeCodexTypes(t *testing.T) {
	svc := newTestService(t)
	for _, pt := range []string{"anthropic", "proxy", "bedrock", "vertex", "open_ai", "config_profile"} {
		if _, err := svc.Add(context.Background(), ProviderInput{Name: "x", ProviderType: pt}); err == nil {
			t.Errorf("Add(providerType=%q) 应被拒绝，claude/codex 已迁移至 agentcfg", pt)
		}
	}
}

// Update 同样不能把一个现役行的类型“升级”回已下线的历史类型。
func TestUpdateRejectsLegacyType(t *testing.T) {
	svc := newTestService(t)
	a := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "gemini"})
	if err := svc.Update(context.Background(), a.ID, ProviderInput{Name: "a", ProviderType: "anthropic"}); err == nil {
		t.Error("Update 把 providerType 改成已下线类型应被拒绝")
	}
}

// List 只返回白名单类型；休眠行（历史遗留/迁移失败兜底，绕过 Add 直接插库模拟）对 List 不可见，
// 但 Get 仍可按 id 直接查到——避免僵尸行永远删不掉。
func TestListFiltersLegacyTypes(t *testing.T) {
	svc := newTestService(t)
	mustAdd(t, svc, ProviderInput{Name: "g", ProviderType: "gemini"})

	enc, err := cryptox.EncryptString("")
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	legacy := storage.Provider{ID: "legacy-cp", Name: "旧配置目录", ProviderType: "config_profile", APIKey: enc}
	if err := svc.db().Create(&legacy).Error; err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ProviderType != "gemini" {
		t.Fatalf("List 应过滤掉休眠类型，got %+v", list)
	}

	got, err := svc.Get(context.Background(), legacy.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get 应仍能查到休眠类型行")
	}
}
