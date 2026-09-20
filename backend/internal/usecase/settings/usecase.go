// Package settings はアプリ設定（タイムゾーン・LLMプロバイダ等）のユースケースを提供する。
package settings

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
)

// DefaultTimezone は未初期化時に返す既定タイムゾーン。
const DefaultTimezone = "Asia/Tokyo"

// LLM プロバイダの選択肢。既定は DeepSeek。
const (
	ProviderDeepSeek = "deepseek"
	ProviderClaude   = "claude"

	// DefaultLLMProvider は未初期化時に返す既定プロバイダ。
	DefaultLLMProvider = ProviderDeepSeek
)

// validProviders は設定として許容する LLM プロバイダの集合。
var validProviders = map[string]struct{}{
	ProviderDeepSeek: {},
	ProviderClaude:   {},
}

// Repository は設定の永続化を抽象化する。
type Repository interface {
	Get(ctx context.Context) (*entity.AppSetting, error)
	Upsert(ctx context.Context, in persistence.AppSettingInput) (*entity.AppSetting, error)
}

// UseCase は設定操作のユースケース。
type UseCase struct {
	repo   Repository
	logger *slog.Logger
}

// New は UseCase を生成する。
func New(repo Repository, logger *slog.Logger) *UseCase {
	return &UseCase{repo: repo, logger: logger}
}

// Get は現在の設定を取得する。未初期化なら既定値（TZ=Asia/Tokyo, provider=deepseek）を返す。
func (u *UseCase) Get(ctx context.Context) (*entity.AppSetting, error) {
	s, err := u.repo.Get(ctx)
	if err != nil {
		if errors.Is(err, derr.ErrNotFound) {
			logging.LogContext(ctx, u.logger, slog.LevelInfo, "settings.get.default")
			return &entity.AppSetting{Timezone: DefaultTimezone, LLMProvider: DefaultLLMProvider}, nil
		}
		return nil, err
	}
	// 既存レコードで provider 未設定（旧データ）なら既定を補う。
	if s.LLMProvider == "" {
		s.LLMProvider = DefaultLLMProvider
	}
	return s, nil
}

// Update はタイムゾーンと LLM プロバイダを検証して設定を更新する。無効値は拒否する。
func (u *UseCase) Update(ctx context.Context, timezone, llmProvider string) (*entity.AppSetting, error) {
	if timezone == "" {
		return nil, derr.NewValidationError("timezone", "timezone is required")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, derr.NewValidationError("timezone", "unknown timezone")
	}
	if llmProvider == "" {
		llmProvider = DefaultLLMProvider
	}
	if _, ok := validProviders[llmProvider]; !ok {
		return nil, derr.NewValidationError("llm_provider", "unknown llm provider")
	}
	logging.LogContext(ctx, u.logger, slog.LevelInfo, "settings.update",
		slog.String("timezone", timezone), slog.String("llm_provider", llmProvider))
	return u.repo.Upsert(ctx, persistence.AppSettingInput{Timezone: timezone, LLMProvider: llmProvider})
}
