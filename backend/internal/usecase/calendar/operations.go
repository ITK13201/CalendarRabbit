package calendar

import (
	"context"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
)

// Create はイベントを作成する。バリデーション後に永続化する。
func (u *UseCase) Create(ctx context.Context, in EventInput) (res *entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, u.logger, "calendar.UseCase.Create", logging.Args{"in": in}, &res, &err)()

	if err = validate(in); err != nil {
		return nil, err
	}
	return u.provider.Create(ctx, toProviderInput(in))
}

// Update は既存イベントを更新する。
func (u *UseCase) Update(ctx context.Context, id int, in EventInput) (res *entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, u.logger, "calendar.UseCase.Update", logging.Args{"id": id, "in": in}, &res, &err)()

	if err = validate(in); err != nil {
		return nil, err
	}
	return u.provider.Update(ctx, id, toProviderInput(in))
}

// Delete は既存イベントを削除する。
func (u *UseCase) Delete(ctx context.Context, id int) (err error) {
	defer logging.Trace(ctx, u.logger, "calendar.UseCase.Delete", logging.Args{"id": id}, nil, &err)()

	return u.provider.Delete(ctx, id)
}

// Get は既存イベントを取得する。
func (u *UseCase) Get(ctx context.Context, id int) (res *entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, u.logger, "calendar.UseCase.Get", logging.Args{"id": id}, &res, &err)()

	return u.provider.Get(ctx, id)
}

// List は全イベントを開始日時順で取得する。
func (u *UseCase) List(ctx context.Context) (res []*entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, u.logger, "calendar.UseCase.List", nil, &res, &err)()

	return u.provider.List(ctx)
}

// ListByPeriod は指定期間 [from, to) に重なるイベントを開始日時順で取得する。
func (u *UseCase) ListByPeriod(ctx context.Context, from, to time.Time) (res []*entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, u.logger, "calendar.UseCase.ListByPeriod",
		logging.Args{"from": from, "to": to}, &res, &err)()

	if err = validatePeriod(from, to); err != nil {
		return nil, err
	}
	return u.provider.ListByPeriod(ctx, from, to)
}
