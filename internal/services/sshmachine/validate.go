package sshmachine

import (
	"errors"
	"strings"
)

// validateSSHField 校验单个 SSH 字段值：非空、不以 '-' 开头（防被 ssh 当作选项）、无首尾空白。
// 对齐 cc-pane validate_ssh_field。
func validateSSHField(value, field string) error {
	t := strings.TrimSpace(value)
	if t == "" {
		return errors.New(field + " cannot be empty")
	}
	if strings.HasPrefix(t, "-") {
		return errors.New(field + " cannot start with '-'")
	}
	if t != value {
		return errors.New(field + " cannot have leading/trailing whitespace")
	}
	return nil
}

// hasDangerousChars 命中 shell 注入相关字符（对齐 cc-pane validate_ssh_machine 的危险字符集）。
// 不含 '\\' / ':'，故 Windows 私钥路径（C:\Users\..\.ssh\id_ed25519）可通过。
func hasDangerousChars(s string) bool {
	return strings.ContainsAny(s, ";|&`$\n\r")
}

// validateInput 校验 Add/Update 的机器输入（对齐 cc-pane validate_ssh_machine）。
// 落库前在 service 层做，是「shell 转义」之外的输入期防御纵深。
func validateInput(in MachineInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("name cannot be empty")
	}
	if err := validateSSHField(in.Host, "host"); err != nil {
		return err
	}
	if hasDangerousChars(in.Host) {
		return errors.New("host contains invalid characters")
	}
	if in.Port != 0 && (in.Port < 1 || in.Port > 65535) {
		return errors.New("port must be between 1 and 65535")
	}
	if in.User != "" && hasDangerousChars(in.User) {
		return errors.New("user contains invalid characters")
	}
	if in.IdentityFile != "" {
		if hasDangerousChars(in.IdentityFile) || strings.Contains(in.IdentityFile, "..") {
			return errors.New("identityFile contains invalid characters")
		}
	}
	if in.DefaultPath != "" {
		if !strings.HasPrefix(in.DefaultPath, "/") && !strings.HasPrefix(in.DefaultPath, "~") {
			return errors.New("defaultPath must start with / or ~")
		}
		if hasDangerousChars(in.DefaultPath) {
			return errors.New("defaultPath contains invalid characters")
		}
	}
	return nil
}
