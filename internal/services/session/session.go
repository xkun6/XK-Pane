// Package session 管理终端会话布局的跨重启恢复（terminal_sessions 表）。
//
// 移植 cc-pane session_restore_repo：save_sessions（**整体替换**：清空 + 批量插入，事务）/
// load_sessions（全量）/ clear_sessions。会话集合是「当前布局快照」，每次保存覆盖上一份。
//
// 安全（SshConfig）：模型 TerminalSession.SshConfig 是敏感字段（SSH 连接配置，可能含主机/凭据），
// 按持久化铁律落库前经 cryptox 加密、读出解密。**SECURITY TODO**（待 Phase 4「运行时 Local/WSL/
// SSH」单元）：sshConfig 是**后端重连数据**而非前端数据；届时应仿 provider 的做法——给前端的
// 绑定方法返回**脱敏**会话（不含明文 sshConfig），仅由非绑定的「SSH 重连」后端 helper 解密使用。
// 当前 SSH 运行时尚未落地、sshConfig 恒为 nil，故暂无明文跨 IPC；本单元只把「加密落库/解密读出」
// 的铁律就位。**本 service 暂不在 app.go 注册**（恢复 UI 属视觉 checkpoint 门控，待 UI 单元再绑）。
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

// LoadSessions 加载全部已保存会话（用于重启恢复布局），按 saved_at 升序。SshConfig 读出后解密。
func (s *Service) LoadSessions(ctx context.Context) ([]storage.TerminalSession, error) {
	rows := make([]storage.TerminalSession, 0)
	if err := s.db().WithContext(ctx).Order("saved_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].SshConfig != nil && *rows[i].SshConfig != "" {
			dec, err := cryptox.DecryptString(*rows[i].SshConfig)
			if err != nil {
				return nil, err
			}
			rows[i].SshConfig = &dec
		}
	}
	return rows, nil
}

// ClearSessions 清空所有已保存会话。
func (s *Service) ClearSessions(ctx context.Context) error {
	return s.db().WithContext(ctx).Where("1 = 1").Delete(&storage.TerminalSession{}).Error
}
