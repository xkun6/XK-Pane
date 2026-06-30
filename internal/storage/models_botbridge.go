package storage

// BotBridgeConfig 保存 IM bot 连接器配置。敏感凭据只存加密后的 TokenEnc；
// BindingsJSON 是 map[platformScopeKey]sessionID，用于按飞书 chat/user 维度绑定会话。
type BotBridgeConfig struct {
	ID           uint   `gorm:"primaryKey;check:id = 1" json:"id"`
	Platform     string `gorm:"size:32;not null;default:'feishu'" json:"platform"`
	AppID        string `gorm:"type:text" json:"appId"`
	TokenEnc     string `gorm:"type:text" json:"-"`
	Enabled      bool   `gorm:"not null;default:false" json:"enabled"`
	BindingsJSON string `gorm:"type:text" json:"bindingsJson"`
	Extra        string `gorm:"type:text" json:"extra"`
	UpdatedAt    int64  `gorm:"autoUpdateTime:milli" json:"updatedAt"`
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
