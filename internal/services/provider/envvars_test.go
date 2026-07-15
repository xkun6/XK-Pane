package provider

import (
	"context"
	"testing"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
)

// ResolveLaunchEnv：一次加载返回 Tier-A env + Tier-B cli provider（含解密明文 key）。
func TestResolveLaunchEnv(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "g", ProviderType: "glm", APIKey: "zk", BaseURL: "zu"})
	env, cli, err := ResolveLaunchEnv(context.Background(), svc.db(), dto.ID)
	if err != nil {
		t.Fatalf("ResolveLaunchEnv: %v", err)
	}
	if env["ZAI_API_KEY"] != "zk" {
		t.Errorf("env ZAI_API_KEY = %q, want zk", env["ZAI_API_KEY"])
	}
	if cli == nil {
		t.Fatal("cli provider should be non-nil")
	}
	if cli.ProviderType != "glm" || cli.APIKey != "zk" || cli.BaseURL != "zu" {
		t.Errorf("cli provider wrong: %+v", cli)
	}
}

func TestResolveLaunchEnvEmptyId(t *testing.T) {
	svc := newTestService(t)
	env, cli, err := ResolveLaunchEnv(context.Background(), svc.db(), "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(env) != 0 || cli != nil {
		t.Fatalf("empty id → empty env + nil cli, got %v / %v", env, cli)
	}
}

func TestEnvVarsEmptyId(t *testing.T) {
	svc := newTestService(t)
	m, _, err := ResolveLaunchEnv(context.Background(), svc.db(), "")
	if err != nil {
		t.Fatalf("ResolveLaunchEnv: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("empty id should yield empty map, got %v", m)
	}
}

func TestEnvVarsUnknownId(t *testing.T) {
	svc := newTestService(t)
	m, _, err := ResolveLaunchEnv(context.Background(), svc.db(), "no-such-id")
	if err != nil {
		t.Fatalf("ResolveLaunchEnv unknown id should not error: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("unknown id should yield empty map, got %v", m)
	}
}

// insertRawProvider 绕过 Service.Add 的类型白名单，直接向 providers 表插入一行（APIKey 走
// cryptox 加密，模拟历史遗留数据——升级前的老用户 / 迁移失败兜底残留）。claude/codex 相关旧
// 类型现已被 Add 拒绝新增（见 provider_test.go 的 TestAddRejectsLegacyClaudeCodexTypes），
// 只能这样构造出对应的行来验证运行期兜底行为。
func insertRawProvider(t *testing.T, svc *Service, p storage.Provider) {
	t.Helper()
	enc, err := cryptox.EncryptString(p.APIKey)
	if err != nil {
		t.Fatalf("cryptox.EncryptString: %v", err)
	}
	p.APIKey = enc
	if err := svc.db().Create(&p).Error; err != nil {
		t.Fatalf("insert raw provider: %v", err)
	}
}

// claude/codex 相关旧类型（anthropic/proxy/bedrock/vertex/config_profile/open_ai）已从
// envVarsFor 退场，与「未知类型」同等对待：ResolveLaunchEnv 必须返回空 env、不报错——stale 的
// LaunchHistory/workspace.providerId 引用不该炸启动。
func TestEnvVarsLegacyTypesYieldEmptyEnv(t *testing.T) {
	svc := newTestService(t)
	cases := []storage.Provider{
		{ID: "legacy-anthropic", Name: "a", ProviderType: "anthropic", APIKey: "sk-ant", BaseURL: "https://api.example.com"},
		{ID: "legacy-proxy", Name: "p", ProviderType: "proxy", APIKey: "sk-proxy", BaseURL: "https://proxy.example.com"},
		{ID: "legacy-bedrock", Name: "b", ProviderType: "bedrock", Region: "us-east-1", AWSProfile: "prod"},
		{ID: "legacy-vertex", Name: "v", ProviderType: "vertex", Region: "us-central1", ProjectID: "proj"},
		{ID: "legacy-config-profile", Name: "c", ProviderType: "config_profile", ConfigDir: "/tmp/dir"},
		{ID: "legacy-openai", Name: "o", ProviderType: "open_ai", APIKey: "k", BaseURL: "u"},
	}
	for _, c := range cases {
		insertRawProvider(t, svc, c)
	}
	for _, c := range cases {
		env, _, err := ResolveLaunchEnv(context.Background(), svc.db(), c.ID)
		if err != nil {
			t.Fatalf("ResolveLaunchEnv(%s): %v", c.ProviderType, err)
		}
		if len(env) != 0 {
			t.Errorf("providerType %q should yield empty env, got %v", c.ProviderType, env)
		}
	}
}

func TestEnvVarsOpencodeUsesOpenAIKey(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "oc", ProviderType: "opencode", APIKey: "k", BaseURL: "u"})
	m, _, _ := ResolveLaunchEnv(context.Background(), svc.db(), dto.ID)
	if m["OPENAI_API_KEY"] != "k" || m["OPENAI_BASE_URL"] != "u" {
		t.Errorf("opencode env wrong: %v", m)
	}
}

func TestEnvVarsGlm(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "g", ProviderType: "glm", APIKey: "zk", BaseURL: "zu"})
	m, _, _ := ResolveLaunchEnv(context.Background(), svc.db(), dto.ID)
	if m["ZAI_API_KEY"] != "zk" || m["ZAI_BASE_URL"] != "zu" {
		t.Errorf("glm env wrong: %v", m)
	}
}

// 空源字段不写入对应变量（faithful：仅 Some 才插入）。
func TestEnvVarsOmitsEmptyBaseUrl(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "k", ProviderType: "kimi", APIKey: "k"})
	m, _, _ := ResolveLaunchEnv(context.Background(), svc.db(), dto.ID)
	if _, ok := m["KIMI_BASE_URL"]; ok {
		t.Error("empty baseUrl should not set KIMI_BASE_URL")
	}
	if m["KIMI_API_KEY"] != "k" {
		t.Errorf("KIMI_API_KEY = %q", m["KIMI_API_KEY"])
	}
}
