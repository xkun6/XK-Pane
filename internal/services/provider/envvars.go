package provider

import (
	"context"
	"errors"

	"xkpane/internal/services/cliadapters"
	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"gorm.io/gorm"
)

// 本文件是 provider 的「启动注入」producer：把持久化的 provider（apiKey 密文）
// 解密后产出 ① Tier-A 启动 env（provider.to_env_vars 中央映射）② Tier-B
// cli provider 上下文（供 glm/cursor/kimi 等 adapter 自注入；kimi 还据此写 per-session 配置）。
//
// ⚠️ 全为**非绑定包级函数**——产出含明文密钥的 env / 上下文，**绝不能进 Wails bindings**
// （故不做成 provider.Service 的导出方法）。仅由后端启动路径（terminal 服务）调用。
//
// 依赖方向：provider → cliadapters（单向、无环；cliadapters 是叶子层不反依赖 provider）。

// loadDecrypted 按 id 加载 provider 并把 APIKey 就地解密成明文（返回的是副本）。
// id=="" 或未知 id → (nil, nil)（faithful：cc-pane warn+empty）。解密失败 → 错误。
func loadDecrypted(ctx context.Context, db *gorm.DB, id string) (*storage.Provider, error) {
	if id == "" {
		return nil, nil
	}
	var p storage.Provider
	err := db.WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	key, err := cryptox.DecryptString(p.APIKey)
	if err != nil {
		return nil, err // 密钥损坏 → 报错（不注入坏 key）
	}
	p.APIKey = key
	return &p, nil
}

// ResolveLaunchEnv 一次加载+解密，返回启动所需的全部：Tier-A env + Tier-B cli provider
// 上下文（含明文 key）。id""/未知 → (空 map, nil, nil)。terminal 启动路径用。
func ResolveLaunchEnv(ctx context.Context, db *gorm.DB, id string) (map[string]string, *cliadapters.Provider, error) {
	p, err := loadDecrypted(ctx, db, id)
	if err != nil {
		return nil, nil, err
	}
	if p == nil {
		return map[string]string{}, nil, nil
	}
	return envVarsFor(p), toCliProvider(p), nil
}

// envVarsFor 是纯映射：解密后的 provider → 启动 env（每变量仅源字段非空才写）。
// codex 易错点：opencode→OPENAI_API_KEY。
//
// claude/codex 相关类型（anthropic/proxy/bedrock/vertex/config_profile/open_ai）已迁移至
// agentcfg（cc-switch 式全局配置 Provider，直写 ~/.claude/settings.json 或
// ~/.codex/config.toml+auth.json，见 internal/services/agentcfg/migrate_legacy.go）并从本
// switch 退场；未命中下面任何 case 的类型（含这些历史类型）与「未知类型」同等对待——返回空
// map、不报错，stale 的 LaunchHistory/workspace.providerId 引用不该炸启动。
func envVarsFor(p *storage.Provider) map[string]string {
	out := map[string]string{}
	if p == nil {
		return out
	}
	set := func(k, v string) {
		if v != "" {
			out[k] = v
		}
	}
	switch p.ProviderType {
	case "gemini":
		set("GEMINI_API_KEY", p.APIKey)
		set("GEMINI_API_BASE", p.BaseURL)
	case "kimi":
		set("KIMI_API_KEY", p.APIKey)
		set("KIMI_BASE_URL", p.BaseURL)
	case "glm":
		set("ZAI_API_KEY", p.APIKey)
		set("ZAI_BASE_URL", p.BaseURL)
	case "cursor":
		set("CURSOR_API_KEY", p.APIKey)
	case "opencode":
		set("OPENAI_API_KEY", p.APIKey)
		set("OPENAI_BASE_URL", p.BaseURL)
	}
	return out
}

// toCliProvider 把解密后的持久化 provider 扁平成 adapter 用的轻量视图（字段 1:1，含明文 key）。
func toCliProvider(p *storage.Provider) *cliadapters.Provider {
	return &cliadapters.Provider{
		ID:           p.ID,
		Name:         p.Name,
		ProviderType: p.ProviderType,
		APIKey:       p.APIKey, // 已解密
		BaseURL:      p.BaseURL,
		Region:       p.Region,
		ProjectID:    p.ProjectID,
		AWSProfile:   p.AWSProfile,
		ConfigDir:    p.ConfigDir,
		IsDefault:    p.IsDefault,
	}
}
