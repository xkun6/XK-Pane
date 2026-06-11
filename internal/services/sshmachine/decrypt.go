package sshmachine

import (
	"context"
	"errors"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"gorm.io/gorm"
)

// ResolveForSSH（**非绑定包级函数**）按 id 加载机器并把 Password 就地解密成明文，供后端
// SSH 启动 / 密码自动应答 helper 使用。⚠️ 返回含明文密码，**绝不能进 Wails bindings**
// （故不做成 Service 的导出方法，仿 provider.ResolveLaunchEnv）。
// id=="" 或未知 id → (nil, nil)；解密失败 → 错误（不返回坏密码）。
func ResolveForSSH(ctx context.Context, db *gorm.DB, id string) (*storage.SshMachine, error) {
	if id == "" {
		return nil, nil
	}
	var m storage.SshMachine
	err := db.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if m.Password != "" {
		pw, err := cryptox.DecryptString(m.Password)
		if err != nil {
			return nil, err
		}
		m.Password = pw
	}
	return &m, nil
}
