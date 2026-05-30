// Package session 管理终端会话布局的跨重启恢复（terminal_sessions 表）。
//
// 移植 cc-pane session_restore_repo：save_sessions（**整体替换**：清空 + 批量插入，事务）/
// load_sessions（全量）/ clear_sessions。会话集合是「当前布局快照」，每次保存覆盖上一份。
//
// 安全（SshConfig）：模型 TerminalSession.SshConfig 是敏感字段（SSH 连接配置，可能含主机/凭据），
// 按持久化铁律落库前经 cryptox 加密、读出解密。sshConfig 是**后端重连数据**而非前端数据，仿
// provider 的做法做**安全分层**：
//   - 绑定方法 `LoadSessions`（前端可达）返回**脱敏**会话（SshConfig 置 nil，绝不把 SSH 配置
//     跨 IPC 交给前端；前端据 RuntimeKind 区分 local/wsl/ssh，无需明文）；
//   - 包级函数 `LoadSessionsDecrypted`（**非绑定**，仿 provider.ResolveLaunchEnv）解密 SshConfig，
//     仅供后端「SSH 重连」helper 使用——Wails 只绑定已注册 service 的方法，包级函数不会被绑定。
// 当前 SSH 运行时尚未落地、sshConfig 恒为 nil，但安全边界先就位，故本 service 现已在 app.go 注册。
package session

import (
	"context"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"gorm.io/gorm"
)

type Service struct {
	holder *storage.Holder
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// SaveSessions 以「整体替换」语义保存当前会话布局快照（清空 + 批量插入，事务）。
// SshConfig 非空则落库前 cryptox 加密。空切片 = 清空。对齐 cc-pane save_sessions。
func (s *Service) SaveSessions(ctx context.Context, sessions []storage.TerminalSession) error {
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&storage.TerminalSession{}).Error; err != nil {
			return err
		}
		if len(sessions) == 0 {
			return nil
		}
		rows := make([]storage.TerminalSession, len(sessions))
		for i, sess := range sessions {
			rows[i] = sess // 值拷贝；不修改调用方切片
			if sess.SshConfig != nil && *sess.SshConfig != "" {
				enc, err := cryptox.EncryptString(*sess.SshConfig)
				if err != nil {
					return err
				}
				rows[i].SshConfig = &enc
			}
		}
		return tx.Create(&rows).Error
	})
}

// LoadSessions（**前端绑定**）加载全部已保存会话用于重启恢复布局，按 saved_at 升序。
// **脱敏**：返回的会话 SshConfig 一律置 nil —— SSH 配置是后端重连数据，绝不跨 IPC 给前端。
// 前端据 RuntimeKind（local/wsl/ssh）即可还原布局，无需明文 SSH 配置。
func (s *Service) LoadSessions(ctx context.Context) ([]storage.TerminalSession, error) {
	return loadSessions(ctx, s.db(), false)
}

// LoadSessionsDecrypted（**包级函数，非绑定**）加载会话并解密 SshConfig，供后端「SSH 重连」
// helper 使用（仿 provider.ResolveLaunchEnv 的非绑定明文出口）。**不暴露给前端**——Wails 仅绑定
// 已注册 service 的方法，包级函数不会被绑定。
func LoadSessionsDecrypted(ctx context.Context, db *gorm.DB) ([]storage.TerminalSession, error) {
	return loadSessions(ctx, db, true)
}

// loadSessions 共享实现：按 saved_at 升序加载。decrypt=true 解密 SshConfig 还原明文（后端重连用）；
// 否则脱敏（SshConfig 置 nil，前端绑定路径用）。
func loadSessions(ctx context.Context, db *gorm.DB, decrypt bool) ([]storage.TerminalSession, error) {
	rows := make([]storage.TerminalSession, 0)
	if err := db.WithContext(ctx).Order("saved_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].SshConfig == nil || *rows[i].SshConfig == "" {
			continue
		}
		if !decrypt {
			rows[i].SshConfig = nil // 脱敏：不把（密文）SSH 配置交给前端
			continue
		}
		dec, err := cryptox.DecryptString(*rows[i].SshConfig)
		if err != nil {
			return nil, err
		}
		rows[i].SshConfig = &dec
	}
	return rows, nil
}

// ClearSessions 清空所有已保存会话。
func (s *Service) ClearSessions(ctx context.Context) error {
	return s.db().WithContext(ctx).Where("1 = 1").Delete(&storage.TerminalSession{}).Error
}
