// Package sshmachine 管理可复用 SSH 远程主机（ssh_machines 表）的 CRUD + 连通性测试。
//
// 移植 cc-pane SshMachineService + SshCredentialService 的合并版。**关键偏离**（均与
// 持久化铁律对齐，同 provider 先例）：
//  1. 持久化：cc-pane 存明文 ssh-machines.json + 密码存 OS keyring；本实现按铁律落 SQLite，
//     Password 经 cryptox 加密（service 层加解密，非 GORM 钩子——读返回脱敏 DTO 无需解密、
//     写为 patch 保留）。无 OS keyring 依赖。
//  2. 脱敏：List/Get 返回 MachineDTO **不含明文 password**（仅 hasStoredPassword 标志）——
//     明文绝不跨 IPC 回前端。
//  3. Update 为 patch：password 输入为空则**保留**原密码（cc-pane remember_password 语义）；
//     ClearStoredPassword=true 或 authMethod≠password 时清空已存密码。
//  4. id 服务端生成 UUID（与 provider/project 一致）。
//
// ResolveForSSH（解密明文出口）在 decrypt.go——非绑定包级函数，绝不进 bindings。
package sshmachine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
	"xkpane/internal/utils/procx"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	errNotFound   = errors.New("SSH machine does not exist")
	errNameExists = errors.New("SSH machine with this name already exists")
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

// MachineDTO 是前端可见的脱敏视图：**不含明文 password**，仅以 hasStoredPassword 表示是否已存。
type MachineDTO struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Host              string    `json:"host"`
	Port              int       `json:"port"`
	User              string    `json:"user"`
	AuthMethod        string    `json:"authMethod"`
	IdentityFile      string    `json:"identityFile"`
	Description       string    `json:"description"`
	DefaultPath       string    `json:"defaultPath"`
	Tags              []string  `json:"tags"`
	HasStoredPassword bool      `json:"hasStoredPassword"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// MachineInput 是前端提交的可写字段。Password 为**明文**：Add 时为空=不存；
// Update 时为空=保留原密码（patch）。ClearStoredPassword=true 显式清空已存密码。
type MachineInput struct {
	Name                string   `json:"name"`
	Host                string   `json:"host"`
	Port                int      `json:"port"`
	User                string   `json:"user"`
	AuthMethod          string   `json:"authMethod"`
	IdentityFile        string   `json:"identityFile"`
	Description         string   `json:"description"`
	DefaultPath         string   `json:"defaultPath"`
	Tags                []string `json:"tags"`
	Password            string   `json:"password"`
	ClearStoredPassword bool     `json:"clearStoredPassword"`
}

// ConnectivityResult 是 CheckConnectivity 的返回（对齐 cc-pane SshConnectivityResult）。
type ConnectivityResult struct {
	Reachable bool   `json:"reachable"`
	Message   string `json:"message"`
	LatencyMs *int64 `json:"latencyMs"`
}

func toDTO(m storage.SshMachine) MachineDTO {
	return MachineDTO{
		ID:                m.ID,
		Name:              m.Name,
		Host:              m.Host,
		Port:              m.Port,
		User:              m.Username,
		AuthMethod:        m.AuthMethod,
		IdentityFile:      m.IdentityFile,
		Description:       m.Description,
		DefaultPath:       m.DefaultPath,
		Tags:              decodeTags(m.Tags),
		HasStoredPassword: m.Password != "", // Password 恒为密文/空；非空即已存
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

func decodeTags(s string) []string {
	out := make([]string, 0)
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	if out == nil {
		out = make([]string, 0)
	}
	return out
}

func encodeTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func authMethodOrDefault(a string) string {
	switch a {
	case "password", "key", "agent":
		return a
	default:
		return "key"
	}
}

func portOrDefault(p int) int {
	if p <= 0 || p > 65535 {
		return 22
	}
	return p
}

// List 返回全部机器（脱敏），按插入序（rowid）——确定性，对齐 cc-pane machines[] 数组序。
func (s *Service) List(ctx context.Context) ([]MachineDTO, error) {
	var rows []storage.SshMachine
	if err := s.db().WithContext(ctx).Order("rowid").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]MachineDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, toDTO(m))
	}
	return out, nil
}

// Get 按 id 查（脱敏）。不存在返回 (nil, nil)。
func (s *Service) Get(ctx context.Context, id string) (*MachineDTO, error) {
	var m storage.SshMachine
	err := s.db().WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dto := toDTO(m)
	return &dto, nil
}

// Add 新增一台机器（服务端生成 id），返回脱敏记录。name 大小写不敏感去重；
// password 明文经 cryptox 加密落库（仅 authMethod==password 且非空时）。
func (s *Service) Add(ctx context.Context, in MachineInput) (MachineDTO, error) {
	if err := validateInput(in); err != nil {
		return MachineDTO{}, err
	}
	auth := authMethodOrDefault(in.AuthMethod)
	pwEnc, err := encryptPasswordFor(auth, in.Password)
	if err != nil {
		return MachineDTO{}, err
	}
	m := storage.SshMachine{
		ID:           uuid.NewString(),
		Name:         strings.TrimSpace(in.Name),
		Host:         in.Host,
		Port:         portOrDefault(in.Port),
		Username:     in.User,
		AuthMethod:   auth,
		IdentityFile: in.IdentityFile,
		Password:     pwEnc,
		Description:  in.Description,
		DefaultPath:  in.DefaultPath,
		Tags:         encodeTags(in.Tags),
	}
	err = s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		exists, err := nameTaken(tx, m.Name, "")
		if err != nil {
			return err
		}
		if exists {
			return errNameExists
		}
		return tx.Create(&m).Error
	})
	if err != nil {
		return MachineDTO{}, err
	}
	return toDTO(m), nil
}

// Update 修改一台机器（patch）。name 去重（排除自身）；password 为空则保留原密码，
// ClearStoredPassword 或 authMethod≠password 时清空。无匹配行报错。
func (s *Service) Update(ctx context.Context, id string, in MachineInput) error {
	if err := validateInput(in); err != nil {
		return err
	}
	auth := authMethodOrDefault(in.AuthMethod)
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur storage.SshMachine
		err := tx.First(&cur, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		taken, err := nameTaken(tx, strings.TrimSpace(in.Name), id)
		if err != nil {
			return err
		}
		if taken {
			return errNameExists
		}
		updates := map[string]any{
			"name":          strings.TrimSpace(in.Name),
			"host":          in.Host,
			"port":          portOrDefault(in.Port),
			"user":          in.User,
			"auth_method":   auth,
			"identity_file": in.IdentityFile,
			"description":   in.Description,
			"default_path":  in.DefaultPath,
			"tags":          encodeTags(in.Tags),
		}
		// 密码 patch：清空 / 改 auth 离开 password → 抹除；显式给新密码 → 覆盖；否则保留原值。
		switch {
		case in.ClearStoredPassword || auth != "password":
			updates["password"] = ""
		case in.Password != "":
			enc, err := cryptox.EncryptString(in.Password)
			if err != nil {
				return err
			}
			updates["password"] = enc
		}
		return tx.Model(&storage.SshMachine{}).Where("id = ?", id).Updates(updates).Error
	})
}

// Remove 删除机器。无匹配行报错（连带其密文密码列一并消失，无独立 keyring 需清理）。
func (s *Service) Remove(ctx context.Context, id string) error {
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&storage.SshMachine{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errNotFound
		}
		return tx.Delete(&storage.SshMachine{}, "id = ?", id).Error
	})
}

// nameTaken 报告（除 excludeID 外）是否已存在同名机器（大小写不敏感）。
func nameTaken(tx *gorm.DB, name, excludeID string) (bool, error) {
	var count int64
	q := tx.Model(&storage.SshMachine{}).Where("lower(name) = ?", strings.ToLower(name))
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// encryptPasswordFor 仅当 authMethod==password 且密码非空时加密，否则返回空（不存密码）。
func encryptPasswordFor(authMethod, password string) (string, error) {
	if authMethod != "password" || password == "" {
		return "", nil
	}
	return cryptox.EncryptString(password)
}

// CheckConnectivity 用 `ssh -o ConnectTimeout=5 -o BatchMode=yes ...` 非交互测试机器可达性
// （对齐 cc-pane check_connectivity）。BatchMode 禁交互密码提示，仅测非交互 reachability；
// 临时 UserKnownHostsFile=null 避免改用户 known_hosts。password 机器命中 auth challenge 视为可达。
func (s *Service) CheckConnectivity(ctx context.Context, id string) (ConnectivityResult, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return ConnectivityResult{}, err
	}
	if m == nil {
		return ConnectivityResult{}, errNotFound
	}
	if err := validateSSHField(m.Host, "host"); err != nil {
		return ConnectivityResult{}, err
	}
	if m.User != "" {
		if err := validateSSHField(m.User, "user"); err != nil {
			return ConnectivityResult{}, err
		}
	}
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return ConnectivityResult{}, errors.New("ssh not found in PATH")
	}

	nullPath := "/dev/null"
	if runtime.GOOS == "windows" {
		nullPath = "NUL"
	}
	args := []string{
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=" + nullPath,
	}
	if m.Port != 22 {
		args = append(args, "-p", strconv.Itoa(m.Port))
	}
	if m.AuthMethod == "key" && m.IdentityFile != "" {
		args = append(args, "-i", m.IdentityFile)
	}
	args = append(args, "--", sshTarget(m.User, m.Host), "exit")

	var stderr strings.Builder
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	start := time.Now()
	runErr := procx.Run(runCtx, procx.Spec{
		Name:          sshPath,
		Args:          args,
		CaptureOutput: true,
		OnStderr:      func(line string) { stderr.WriteString(line + "\n") },
	})
	latency := time.Since(start).Milliseconds()

	if runErr == nil {
		return ConnectivityResult{Reachable: true, Message: fmt.Sprintf("Connected in %dms", latency), LatencyMs: &latency}, nil
	}
	errText := stderr.String()
	if m.AuthMethod == "password" && looksLikeAuthChallenge(errText) {
		return ConnectivityResult{
			Reachable: true,
			Message:   "SSH host reachable; password/MFA were not validated in connectivity test",
			LatencyMs: &latency,
		}, nil
	}
	msg := firstLine(errText)
	if msg == "" {
		msg = "Connection failed"
	}
	return ConnectivityResult{Reachable: false, Message: msg, LatencyMs: nil}, nil
}

// sshTarget 拼 user@host 或仅 host。
func sshTarget(user, host string) string {
	if user != "" {
		return user + "@" + host
	}
	return host
}

func firstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			return ln
		}
	}
	return ""
}

// looksLikeAuthChallenge 据 stderr 判断「主机可达但需认证」（对齐 cc-pane looks_like_auth_challenge）。
func looksLikeAuthChallenge(stderr string) bool {
	l := strings.ToLower(stderr)
	return strings.Contains(l, "permission denied") ||
		strings.Contains(l, "password:") ||
		strings.Contains(l, "keyboard-interactive") ||
		strings.Contains(l, "verification code")
}
