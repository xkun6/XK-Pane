package storage

// BotBridgeConfig 保存 IM bot 连接器配置。
// 凭据存加密后的 CredentialsEnc（各平台凭据字段的 map[string]string 加密 JSON），各平台字段不同
// （飞书 app_id/app_secret、钉钉 client_id/client_secret、企业微信 bot_id/bot_secret…）。
// 旧版只有 AppID + TokenEnc（飞书单平台），两列保留用于向后兼容读取：CredentialsEnc 为空时
// loadConfig 回落把 {app_id:AppID, app_secret:解密(TokenEnc)} 合成出来。
// BindingsJSON 是 map[platformScopeKey]sessionID，用于按 chat/user 维度绑定会话。
type BotBridgeConfig struct {
	ID             uint   `gorm:"primaryKey;check:id = 1" json:"id"`
	Platform       string `gorm:"size:32;not null;default:'feishu'" json:"platform"`
	AppID          string `gorm:"type:text" json:"appId"`
	TokenEnc       string `gorm:"type:text" json:"-"`
	CredentialsEnc string `gorm:"type:text" json:"-"`
	Enabled        bool   `gorm:"not null;default:false" json:"enabled"`
	BindingsJSON   string `gorm:"type:text" json:"bindingsJson"`
	Extra          string `gorm:"type:text" json:"extra"`
	UpdatedAt      int64  `gorm:"autoUpdateTime:milli" json:"updatedAt"`
}

func (BotBridgeConfig) TableName() string { return "bot_bridge_config" }

func init() {
	AllModels = append(AllModels, ModelDescriptor{
		Model:     &BotBridgeConfig{},
		TableName: "bot_bridge_config",
		LabelKey:  "botBridge",
		Clearable: false,
	})
}
