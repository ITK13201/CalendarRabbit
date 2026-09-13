// Package calendar はカレンダーイベントのユースケース（CRUD・期間取得）を提供する。
package calendar

import (
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/service/calendarprovider"
)

// UseCase はカレンダー操作のユースケース。CalendarProvider 経由で永続化する。
type UseCase struct {
	provider calendarprovider.CalendarProvider
	logger   *slog.Logger
}

// New は UseCase を生成する。
func New(provider calendarprovider.CalendarProvider, logger *slog.Logger) *UseCase {
	return &UseCase{provider: provider, logger: logger}
}

// EventInput はイベントの作成・更新入力。
type EventInput struct {
	Title       string
	StartsAt    time.Time
	EndsAt      time.Time
	AllDay      bool
	Location    string
	Description string
	SourceURL   string
}

func toProviderInput(in EventInput) calendarprovider.EventInput {
	return calendarprovider.EventInput{
		Title:       in.Title,
		StartsAt:    in.StartsAt,
		EndsAt:      in.EndsAt,
		AllDay:      in.AllDay,
		Location:    in.Location,
		Description: in.Description,
		SourceURL:   in.SourceURL,
	}
}
