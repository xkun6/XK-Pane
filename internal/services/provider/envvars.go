package provider

import (
	"context"
	"errors"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"gorm.io/gorm"
)

// EnvVars 复刻 cc-pane provider.to_env_vars + resolve_env_vars：按 provider 类型产出
// 启动 CLI 用的环境变量（含**解密后的明文 apiKey**）。
//
// ⚠️ 这是**非绑定包级函数**——它产出含明文密钥的 env，**绝不能进 Wails bindings**
// （故不做成 provider.Service 的导出方法）。仅由后端启动路径（terminal 服务）调用。
//
// 语义（faithful cc-pane）：id=="" 或未知 id → 空 map（无注入，不报错）；已知 id →
// 解密 apiKey 后按 ProviderType 产出 env，每个变量仅在其源字段非空时写入。
// 密钥解密失败（主密钥损坏/换机）→ 返回错误，由调用方决定降级（不静默注入坏 key）。
//
// 注意 codex 易错点：open_ai(codex)→CODEX_API_KEY（非 OPENAI_API_KEY）；
// opencode→OPENAI_API_KEY。config_profile 仅做 dir-mode（CLAUDE_CONFIG_DIR）；
// file-mode（解析 ccswitch JSON 的 env{}）延后（需 filex 限长读，Phase 5）。
func EnvVars(ctx context.Context, db *gorm.DB, id string) (map[string]string, error) {
	out := map[string]string{}
	if id == "" {
		return out, nil
	}
	var p storage.Provider
	err := db.WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, nil // 未知 id → 空（faithful：cc-pane warn+empty）
	}
	if err != nil {
		return nil, err
	}
	apiKey, err := cryptox.DecryptString(p.APIKey)
	if err != nil {
		return nil, err // 密钥损坏 → 报错（不注入坏 key）
	}
	set := func(k, v string) {
		if v != "" {
			out[k] = v
		}
	}
	switch p.ProviderType {
	case "anthropic", "proxy":
		set("ANTHROPIC_API_KEY", apiKey)
		set("ANTHROPIC_BASE_URL", p.BaseURL)
	case "bedrock":
		out["CLAUDE_CODE_USE_BEDROCK"] = "1"
		set("AWS_REGION", p.Region)
		set("AWS_PROFILE", p.AWSProfile)
	case "vertex":
		out["CLAUDE_CODE_USE_VERTEX"] = "1"
		set("CLOUD_ML_REGION", p.Region)
		set("ANTHROPIC_VERTEX_PROJECT_ID", p.ProjectID)
	case "config_profile":
		set("CLAUDE_CONFIG_DIR", p.ConfigDir) // 仅 dir-mode；file-mode 延后
	case "open_ai":
		set("CODEX_API_KEY", apiKey) // codex 用 CODEX_API_KEY，非 OPENAI_API_KEY
		set("OPENAI_BASE_URL", p.BaseURL)
	case "gemini":
		set("GEMINI_API_KEY", apiKey)
		set("GEMINI_API_BASE", p.BaseURL)
	case "kimi":
		set("KIMI_API_KEY", apiKey)
		set("KIMI_BASE_URL", p.BaseURL)
	case "glm":
		set("ZAI_API_KEY", apiKey)
		set("ZAI_BASE_URL", p.BaseURL)
	case "cursor":
		set("CURSOR_API_KEY", apiKey)
	case "opencode":
		set("OPENAI_API_KEY", apiKey)
		set("OPENAI_BASE_URL", p.BaseURL)
	}
	return out, nil
}
