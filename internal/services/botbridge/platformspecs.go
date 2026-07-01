package botbridge

// CredentialField 描述一个平台的凭据字段。Secret=true 的字段前端用密码框渲染、
// 状态查询不回显其值、保存时留空即沿用旧值。
type CredentialField struct {
	Key    string `json:"key"`
	Secret bool   `json:"secret"`
}

// PlatformSpec 描述一个 IM 平台接入所需的凭据字段。前端配置页据此动态渲染输入框
//（字段标签由前端按 i18n key botbridge.field.<key> 翻译），后端据此判定哪些字段敏感。
type PlatformSpec struct {
	ID     string            `json:"id"`
	Fields []CredentialField `json:"fields"`
}

// platformSpecs 是所有「已知」平台的凭据字段规格（静态表）。某平台是否真的可选，
// 取决于其适配器有没有 init() 注册（见 registeredPlatformSpecs）。
var platformSpecs = []PlatformSpec{
	{ID: "feishu", Fields: []CredentialField{{Key: "app_id"}, {Key: "app_secret", Secret: true}}},
	{ID: "qqbot", Fields: []CredentialField{{Key: "app_id"}, {Key: "app_secret", Secret: true}}},
	{ID: "dingtalk", Fields: []CredentialField{
		{Key: "client_id"}, {Key: "client_secret", Secret: true}, {Key: "robot_code"},
	}},
	{ID: "wecom", Fields: []CredentialField{{Key: "bot_id"}, {Key: "bot_secret", Secret: true}}},
	{ID: "qq", Fields: []CredentialField{{Key: "ws_url"}, {Key: "token", Secret: true}}},
	{ID: "weixin", Fields: []CredentialField{{Key: "token", Secret: true}}},
}

func platformSpec(id string) (PlatformSpec, bool) {
	for _, sp := range platformSpecs {
		if sp.ID == id {
			return sp, true
		}
	}
	return PlatformSpec{}, false
}

// registeredPlatformSpecs 仅返回「已注册适配器」的平台规格——未接线的平台不暴露给前端，
// 避免用户选了一个没有适配器的平台导致保存即失败。随各平台 init() 注册而增多。
func registeredPlatformSpecs() []PlatformSpec {
	out := make([]PlatformSpec, 0, len(platformSpecs))
	for _, sp := range platformSpecs {
		if _, ok := platformFactories[sp.ID]; ok {
			out = append(out, sp)
		}
	}
	return out
}

// isSecretField 判断某平台的某凭据键是否敏感。未知平台/未知字段按敏感处理，避免误明文回显。
func isSecretField(platform, key string) bool {
	sp, ok := platformSpec(platform)
	if !ok {
		return true
	}
	for _, f := range sp.Fields {
		if f.Key == key {
			return f.Secret
		}
	}
	return true
}
