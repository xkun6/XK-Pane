package terminal

import "github.com/google/uuid"

// newSessionID 生成会话唯一 ID。与 cc-pane 一致用 UUID v4 字符串
// （cc-panes-core/src/services/terminal_service.rs: `Uuid::new_v4().to_string()`）。
func newSessionID() string {
	return uuid.NewString()
}
