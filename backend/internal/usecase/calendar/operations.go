package calendar

import (
	"context"
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
)

// Create はイベントを作成する。バリデーション後に永続化する。
func (u *UseCase) Create(ctx context.Context, in EventInput) (*entity.CalendarEvent, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	logging.LogContext(ctx, u.logger, slog.LevelInfo, "calendar.create", slog.String("title", in.Title))
	return u.provider.Create(ctx, toProviderInput(in))
}

// Update は既存イベントを更新する。
func (u *UseCase) Update(ctx context.Context, id int, in EventInput) (*entity.CalendarEvent, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	logging.LogContext(ctx, u.logger, slog.LevelInfo, "calendar.update", slog.Int("id", id))
	return u.provider.Update(ctx, id, toProviderInput(in))
}

// Delete は既存イベントを削除する。
func (u *UseCase) Delete(ctx context.Context, id int) error {
	logging.LogContext(ctx, u.logger, slog.LevelInfo, "calendar.delete", slog.Int("id", id))
	return u.provider.Delete(ctx, id)
}

// Get は既存イベントを取得する。
func (u *UseCase) Get(ctx context.Context, id int) (*entity.CalendarEvent, error) {
	logging.LogContext(ctx, u.logger, slog.LevelInfo, "calendar.get", slog.Int("id", id))
	return u.provider.Get(ctx, id)
}

// List は全イベントを開始日時順で取得する。
func (u *UseCase) List(ctx context.Context) ([]*entity.CalendarEvent, error) {
	logging.LogContext(ctx, u.logger, slog.LevelInfo, "calendar.list")
	return u.provider.List(ctx)
}

// ListByPeriod は指定期間 [from, to) に重なるイベントを開始日時順で取得する。
func (u *UseCase) ListByPeriod(ctx context.Context, from, to time.Time) ([]*entity.CalendarEvent, error) {
	if err := validatePeriod(from, to); err != nil {
		return nil, err
	}
	logging.LogContext(ctx, u.logger, slog.LevelInfo, "calendar.list_by_period",
		slog.Time("from", from), slog.Time("to", to))
	return u.provider.ListByPeriod(ctx, from, to)
}
