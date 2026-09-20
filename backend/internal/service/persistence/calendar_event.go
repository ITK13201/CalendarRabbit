package persistence

import (
	"context"
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/ent/calendarevent"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
)

// CalendarEventRepository は CalendarEvent の永続化を担う。
type CalendarEventRepository struct {
	client *ent.Client
	logger *slog.Logger
}

// NewCalendarEventRepository は CalendarEventRepository を生成する。
func NewCalendarEventRepository(client *ent.Client, logger *slog.Logger) *CalendarEventRepository {
	return &CalendarEventRepository{client: client, logger: logger}
}

// CalendarEventInput は作成・更新の入力。
type CalendarEventInput struct {
	Title       string
	StartsAt    time.Time
	EndsAt      time.Time
	AllDay      bool
	Location    string
	Description string
	SourceURL   string
}

func (r *CalendarEventRepository) Create(ctx context.Context, in CalendarEventInput) (res *entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, r.logger, "persistence.CalendarEventRepository.Create", logging.Args{"in": in}, &res, &err)()

	row, err := r.client.CalendarEvent.Create().
		SetTitle(in.Title).
		SetStartsAt(in.StartsAt.UTC()).
		SetEndsAt(in.EndsAt.UTC()).
		SetAllDay(in.AllDay).
		SetLocation(in.Location).
		SetDescription(in.Description).
		SetSourceURL(in.SourceURL).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return mapCalendarEvent(row), nil
}

func (r *CalendarEventRepository) Get(ctx context.Context, id int) (res *entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, r.logger, "persistence.CalendarEventRepository.Get", logging.Args{"id": id}, &res, &err)()

	row, err := r.client.CalendarEvent.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, derr.ErrNotFound
		}
		return nil, err
	}
	return mapCalendarEvent(row), nil
}

func (r *CalendarEventRepository) Update(ctx context.Context, id int, in CalendarEventInput) (res *entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, r.logger, "persistence.CalendarEventRepository.Update", logging.Args{"id": id, "in": in}, &res, &err)()

	row, err := r.client.CalendarEvent.UpdateOneID(id).
		SetTitle(in.Title).
		SetStartsAt(in.StartsAt.UTC()).
		SetEndsAt(in.EndsAt.UTC()).
		SetAllDay(in.AllDay).
		SetLocation(in.Location).
		SetDescription(in.Description).
		SetSourceURL(in.SourceURL).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, derr.ErrNotFound
		}
		return nil, err
	}
	return mapCalendarEvent(row), nil
}

func (r *CalendarEventRepository) Delete(ctx context.Context, id int) (err error) {
	defer logging.Trace(ctx, r.logger, "persistence.CalendarEventRepository.Delete", logging.Args{"id": id}, nil, &err)()

	err = r.client.CalendarEvent.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return derr.ErrNotFound
		}
		return err
	}
	return nil
}

// ListByPeriod は指定期間 [from, to) に重なるイベントを開始日時順で返す。
// 重なり条件: starts_at < to かつ ends_at > from（複数日イベントを包含）。
func (r *CalendarEventRepository) ListByPeriod(ctx context.Context, from, to time.Time) (res []*entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, r.logger, "persistence.CalendarEventRepository.ListByPeriod",
		logging.Args{"from": from, "to": to}, &res, &err)()

	rows, err := r.client.CalendarEvent.Query().
		Where(
			calendarevent.StartsAtLT(to.UTC()),
			calendarevent.EndsAtGT(from.UTC()),
		).
		Order(ent.Asc(calendarevent.FieldStartsAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return mapCalendarEvents(rows), nil
}

// List は全イベントを開始日時順で返す。
func (r *CalendarEventRepository) List(ctx context.Context) (res []*entity.CalendarEvent, err error) {
	defer logging.Trace(ctx, r.logger, "persistence.CalendarEventRepository.List", nil, &res, &err)()

	rows, err := r.client.CalendarEvent.Query().
		Order(ent.Asc(calendarevent.FieldStartsAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return mapCalendarEvents(rows), nil
}

func mapCalendarEvent(row *ent.CalendarEvent) *entity.CalendarEvent {
	return &entity.CalendarEvent{
		ID:          row.ID,
		Title:       row.Title,
		StartsAt:    row.StartsAt.UTC(),
		EndsAt:      row.EndsAt.UTC(),
		AllDay:      row.AllDay,
		Location:    row.Location,
		Description: row.Description,
		SourceURL:   row.SourceURL,
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
}

func mapCalendarEvents(rows []*ent.CalendarEvent) []*entity.CalendarEvent {
	out := make([]*entity.CalendarEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapCalendarEvent(row))
	}
	return out
}
