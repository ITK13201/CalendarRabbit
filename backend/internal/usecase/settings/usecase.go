// Package settings はアプリ設定（タイムゾーン等）のユースケースを提供する。
package settings

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
)

// DefaultTimezone は未初期化時に返す既定タイムゾーン。
const DefaultTimezone = "Asia/Tokyo"

// Repository は設定の永続化を抽象化する。
type Repository interface {
	Get(ctx context.Context) (*entity.AppSetting, error)
	Upsert(ctx context.Context, timezone string) (*entity.AppSetting, error)
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

// Get は現在の設定を取得する。未初期化なら既定値（TZ=Asia/Tokyo）を返す。
func (u *UseCase) Get(ctx context.Context) (*entity.AppSetting, error) {
	s, err := u.repo.Get(ctx)
	if err != nil {
		if errors.Is(err, derr.ErrNotFound) {
			logging.LogContext(ctx, u.logger, slog.LevelInfo, "settings.get.default")
			return &entity.AppSetting{Timezone: DefaultTimezone}, nil
		}
		return nil, err
	}
	return s, nil
}

// Update はタイムゾーンを検証して設定を更新する。無効なTZは拒否する。
func (u *UseCase) Update(ctx context.Context, timezone string) (*entity.AppSetting, error) {
	if timezone == "" {
		return nil, derr.NewValidationError("timezone", "timezone is required")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, derr.NewValidationError("timezone", "unknown timezone")
	}
	logging.LogContext(ctx, u.logger, slog.LevelInfo, "settings.update", slog.String("timezone", timezone))
	return u.repo.Upsert(ctx, timezone)
}
