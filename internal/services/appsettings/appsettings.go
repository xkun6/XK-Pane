// Package appsettings 提供"应用设置"单行表的读写。
//
// 单行表（CHECK id=1）保存"必有且唯一"的配置项：
// 主题选择、自定义主题 JSON、语言选择。
//
// 与 preferences 的区别：
//   - 这里字段是已知有限集，schema 强校验
//   - preferences 是开放键值，业务方可以随时塞新键
package appsettings

import (
	"context"

	"xkpane/internal/storage"

	"gorm.io/gorm"
)

type Service struct {
	holder         *storage.Holder
	onLocaleChange func(string) // 语言变更后回调（托盘菜单据此重建文案）；可空，启动期注册一次
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// Snapshot 是对外暴露的整体快照，前端启动时一次拉取。
type Snapshot struct {
	ThemeChoice  string `json:"themeChoice"`
	CustomTheme  string `json:"customTheme"` // JSON: { mode, palette }；空字符串=未设置
	LocaleChoice string `json:"localeChoice"`
}

// Get 一次性返回完整快照。
func (s *Service) Get(ctx context.Context) (Snapshot, error) {
	var row storage.AppSettings
	if err := s.db().WithContext(ctx).First(&row, "id = ?", 1).Error; err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		ThemeChoice:  row.ThemeChoice,
		CustomTheme:  row.CustomTheme,
		LocaleChoice: row.LocaleChoice,
	}, nil
}

// SetThemeChoice 仅更新主题选择。
func (s *Service) SetThemeChoice(ctx context.Context, choice string) error {
	return s.db().WithContext(ctx).
		Model(&storage.AppSettings{}).
		Where("id = ?", 1).
		Update("theme_choice", choice).Error
}

// SetCustomTheme 写入自定义主题 JSON（前端 stringify 后传入）。
// 传空字符串等价于"未设置"。
func (s *Service) SetCustomTheme(ctx context.Context, json string) error {
	return s.db().WithContext(ctx).
		Model(&storage.AppSettings{}).
		Where("id = ?", 1).
		Update("custom_theme", json).Error
}

// ResetCustomTheme 清空自定义主题。
func (s *Service) ResetCustomTheme(ctx context.Context) error {
	return s.SetCustomTheme(ctx, "")
}

// SetLocaleChoice 'auto' | LocaleCode。写库成功后触发 onLocaleChange（托盘菜单据此重建文案）。
func (s *Service) SetLocaleChoice(ctx context.Context, choice string) error {
	if err := s.db().WithContext(ctx).
		Model(&storage.AppSettings{}).
		Where("id = ?", 1).
		Update("locale_choice", choice).Error; err != nil {
		return err
	}
	if s.onLocaleChange != nil {
		s.onLocaleChange(choice)
	}
	return nil
}

// SetLocaleChangeHook 注册语言变更回调（SetLocaleChoice 写库成功后触发）。
// 当前用途：托盘原生菜单据新语言重建文案。仅启动期注册一次（非并发安全）。
func (s *Service) SetLocaleChangeHook(fn func(string)) {
	s.onLocaleChange = fn
}
