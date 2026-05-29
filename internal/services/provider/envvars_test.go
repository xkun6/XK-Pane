package provider

import (
	"context"
	"testing"
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
	m, err := EnvVars(context.Background(), svc.db(), "")
	if err != nil {
		t.Fatalf("EnvVars: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("empty id should yield empty map, got %v", m)
	}
}

func TestEnvVarsUnknownId(t *testing.T) {
	svc := newTestService(t)
	m, err := EnvVars(context.Background(), svc.db(), "no-such-id")
	if err != nil {
		t.Fatalf("EnvVars unknown id should not error: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("unknown id should yield empty map, got %v", m)
	}
}

// 加密落库 → EnvVars 解密注入 的完整往返：apiKey 值应是明文。
func TestEnvVarsAnthropicDecryptsKey(t *testing.T) {
	svc := newTestService(t)
	const key = "sk-ant-secret"
	dto := mustAdd(t, svc, ProviderInput{
		Name: "a", ProviderType: "anthropic", APIKey: key, BaseURL: "https://api.example.com",
	})
	m, err := EnvVars(context.Background(), svc.db(), dto.ID)
	if err != nil {
		t.Fatalf("EnvVars: %v", err)
	}
	if m["ANTHROPIC_API_KEY"] != key {
		t.Errorf("ANTHROPIC_API_KEY = %q, want decrypted %q", m["ANTHROPIC_API_KEY"], key)
	}
	if m["ANTHROPIC_BASE_URL"] != "https://api.example.com" {
		t.Errorf("ANTHROPIC_BASE_URL = %q", m["ANTHROPIC_BASE_URL"])
	}
}

func TestEnvVarsBedrockNoKey(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{
		Name: "b", ProviderType: "bedrock", Region: "us-east-1", AWSProfile: "prod",
	})
	m, _ := EnvVars(context.Background(), svc.db(), dto.ID)
	if m["CLAUDE_CODE_USE_BEDROCK"] != "1" {
		t.Errorf("CLAUDE_CODE_USE_BEDROCK = %q, want 1", m["CLAUDE_CODE_USE_BEDROCK"])
	}
	if m["AWS_REGION"] != "us-east-1" || m["AWS_PROFILE"] != "prod" {
		t.Errorf("bedrock env wrong: %v", m)
	}
	if _, ok := m["ANTHROPIC_API_KEY"]; ok {
		t.Error("bedrock should not set ANTHROPIC_API_KEY")
	}
}

// codex 易错点：open_ai → CODEX_API_KEY（非 OPENAI_API_KEY）。
func TestEnvVarsOpenAIUsesCodexKey(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "x", ProviderType: "open_ai", APIKey: "k", BaseURL: "u"})
	m, _ := EnvVars(context.Background(), svc.db(), dto.ID)
	if m["CODEX_API_KEY"] != "k" {
		t.Errorf("open_ai must set CODEX_API_KEY=k, got %v", m)
	}
	if _, ok := m["OPENAI_API_KEY"]; ok {
		t.Error("open_ai must NOT set OPENAI_API_KEY (that's opencode)")
	}
	if m["OPENAI_BASE_URL"] != "u" {
		t.Errorf("OPENAI_BASE_URL = %q", m["OPENAI_BASE_URL"])
	}
}

func TestEnvVarsOpencodeUsesOpenAIKey(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "oc", ProviderType: "opencode", APIKey: "k", BaseURL: "u"})
	m, _ := EnvVars(context.Background(), svc.db(), dto.ID)
	if m["OPENAI_API_KEY"] != "k" || m["OPENAI_BASE_URL"] != "u" {
		t.Errorf("opencode env wrong: %v", m)
	}
}

func TestEnvVarsGlm(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "g", ProviderType: "glm", APIKey: "zk", BaseURL: "zu"})
	m, _ := EnvVars(context.Background(), svc.db(), dto.ID)
	if m["ZAI_API_KEY"] != "zk" || m["ZAI_BASE_URL"] != "zu" {
		t.Errorf("glm env wrong: %v", m)
	}
}

// 空源字段不写入对应变量（faithful：仅 Some 才插入）。
func TestEnvVarsOmitsEmptyBaseUrl(t *testing.T) {
	svc := newTestService(t)
	dto := mustAdd(t, svc, ProviderInput{Name: "a", ProviderType: "anthropic", APIKey: "k"})
	m, _ := EnvVars(context.Background(), svc.db(), dto.ID)
	if _, ok := m["ANTHROPIC_BASE_URL"]; ok {
		t.Error("empty baseUrl should not set ANTHROPIC_BASE_URL")
	}
	if m["ANTHROPIC_API_KEY"] != "k" {
		t.Errorf("ANTHROPIC_API_KEY = %q", m["ANTHROPIC_API_KEY"])
	}
}
