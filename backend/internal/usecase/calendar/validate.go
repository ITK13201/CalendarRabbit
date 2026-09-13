package calendar

import (
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
)

// validate は作成・更新共通のバリデーションを行う。
// 必須項目（名称・開始日時）欠如、終了<開始をエラーとする。
func validate(in EventInput) error {
	if in.Title == "" {
		return derr.NewValidationError("title", "title is required")
	}
	if in.StartsAt.IsZero() {
		return derr.NewValidationError("starts_at", "starts_at is required")
	}
	if in.EndsAt.IsZero() {
		return derr.NewValidationError("ends_at", "ends_at is required")
	}
	if in.EndsAt.Before(in.StartsAt) {
		return derr.NewValidationError("ends_at", "ends_at must not be before starts_at")
	}
	return nil
}

// validatePeriod は期間取得の from/to を検証する。
func validatePeriod(from, to time.Time) error {
	if from.IsZero() {
		return derr.NewValidationError("from", "from is required")
	}
	if to.IsZero() {
		return derr.NewValidationError("to", "to is required")
	}
	if !to.After(from) {
		return derr.NewValidationError("to", "to must be after from")
	}
	return nil
}
