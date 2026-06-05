package storage

import "time"

// 本文件移植 cc-pane 的 SSH 机器模型（ssh_machine_service / ssh_credential_service）。
//
// **偏离说明**：cc-pane 把机器存成明文 ssh-machines.json + 密码存 OS keyring；
// XK-Pane 按持久化铁律落 SQLite，且 Password **以 cryptox 密文存储**（在 sshmachine
// service 层加解密，不走 GORM 钩子——读路径返回脱敏 DTO 无需解密、写路径是 patch 保留）。
// 与 Provider.APIKey 同一偏离先例。
//   - IdentityFile 是本机私钥文件**路径**（非密钥内容），不敏感、不加密——对齐 cc-pane
//     identity_file 语义（ssh -i 直接读该文件）。
//   - 不设 is_default：机器为「每次启动时挑选」，无默认机器概念（对齐 cc-pane）。

func init() {
	AllModels = append(AllModels,
		// 含敏感凭据（password 密文）→ Clearable:false（禁设置页一键清空，同 providers 档）。
		ModelDescriptor{Model: &SshMachine{}, TableName: "ssh_machines", LabelKey: "sshMachines", Clearable: false},
	)
}

// SshMachine 是一台可复用的 SSH 远程主机（移植自 cc-pane SshMachine）。
// Name 全局唯一（service 层做大小写不敏感去重）。Password 恒为密文/空：消费方读出后须
// cryptox.DecryptString；前端绝不拿到明文（List/Get 返回脱敏 DTO，仅 hasStoredPassword 标志）。
type SshMachine struct {
	ID           string `gorm:"primaryKey;size:36" json:"id"`
	Name         string `gorm:"type:text;not null;uniqueIndex" json:"name"`
	Host         string `gorm:"type:text;not null" json:"host"`
	Port         int    `gorm:"not null;default:22" json:"port"`
	Username     string `gorm:"type:text;column:user" json:"user,omitempty"`              // 对齐 cc-pane user
	AuthMethod   string `gorm:"type:text;not null;default:'key';column:auth_method" json:"authMethod"` // password | key | agent
	IdentityFile string `gorm:"type:text;column:identity_file" json:"identityFile,omitempty"`          // 私钥文件路径（非内容，不加密）
	Password     string `gorm:"type:text" json:"-"`                                       // 密文（cryptox）；json:"-" 防序列化；"" = 未存
	Description  string `gorm:"type:text" json:"description,omitempty"`
	DefaultPath  string `gorm:"type:text;column:default_path" json:"defaultPath,omitempty"` // 远端默认 cd 目录；为空用 ~
	Tags         string `gorm:"type:text;not null;default:'[]'" json:"tags"`               // JSON 数组（service 层与前端按字符串数组用）

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (SshMachine) TableName() string { return "ssh_machines" }
