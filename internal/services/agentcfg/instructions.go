package agentcfg

// 本文件实现「全局指令」文件的快捷读写：Claude Code 的 <home>/.claude/CLAUDE.md 与
// Codex 的 <home>/.codex/AGENTS.md。这两个文件是纯文本（Markdown），不像 settings.json
// / config.toml 那样有结构化 Provider 概念，这里只做「读全文 / 写全文」两个薄方法，供
// 设置页的全局指令编辑器直接使用，不接入 agent_providers 表。

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"xkpane/internal/utils/filex"
)

// instructionFilePath 返回 appType 对应的全局指令文件路径：
// claude → <home>/.claude/CLAUDE.md；codex → <home>/.codex/AGENTS.md。
func (s *Service) instructionFilePath(appType string) string {
	if appType == appTypeCodex {
		return filepath.Join(s.home, ".codex", "AGENTS.md")
	}
	return filepath.Join(s.home, ".claude", "CLAUDE.md")
}

// ReadInstructionFile 读全局指令文件全文。文件不存在 → 返回空串（不是错误，首次使用
// 本功能时文件可能还没建）。上限 8MB，防止意外读入巨型文件。
func (s *Service) ReadInstructionFile(appType string) (string, error) {
	if err := validateAppType(appType); err != nil {
		return "", err
	}
	path := s.instructionFilePath(appType)
	data, err := filex.ReadLimit(path, 8<<20)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("agentcfg: 读取 %s 失败: %w", path, err)
	}
	return string(data), nil
}

// WriteInstructionFile 写全局指令文件：先确保父目录存在，首写前 backupLiveOnce 留一份
// .xkpane-bak，再原子写入。
func (s *Service) WriteInstructionFile(appType string, content string) error {
	if err := validateAppType(appType); err != nil {
		return err
	}
	path := s.instructionFilePath(appType)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("agentcfg: 创建 %s 失败: %w", filepath.Dir(path), err)
	}
	if err := backupLiveOnce(path); err != nil {
		return fmt.Errorf("agentcfg: 备份 live 指令文件失败: %w", err)
	}
	if err := filex.WriteAtomic(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("agentcfg: 写入 %s 失败: %w", path, err)
	}
	return nil
}
