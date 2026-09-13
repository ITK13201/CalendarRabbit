package handler_test

import (
	"context"
	"strconv"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/calendarprovider"
)

func itoa(i int) string { return strconv.Itoa(i) }

// memProvider は CalendarProvider のインメモリ実装（handler テスト用）。
type memProvider struct {
	events map[int]*entity.CalendarEvent
	nextID int
}

func newMemProvider() *memProvider {
	return &memProvider{events: map[int]*entity.CalendarEvent{}, nextID: 1}
}

func (m *memProvider) Create(_ context.Context, in calendarprovider.EventInput) (*entity.CalendarEvent, error) {
	e := &entity.CalendarEvent{
		ID: m.nextID, Title: in.Title, StartsAt: in.StartsAt.UTC(), EndsAt: in.EndsAt.UTC(),
		AllDay: in.AllDay, Location: in.Location, Description: in.Description, SourceURL: in.SourceURL,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	m.events[m.nextID] = e
	m.nextID++
	return e, nil
}

func (m *memProvider) Update(_ context.Context, id int, in calendarprovider.EventInput) (*entity.CalendarEvent, error) {
	e, ok := m.events[id]
	if !ok {
		return nil, derr.ErrNotFound
	}
	e.Title, e.StartsAt, e.EndsAt = in.Title, in.StartsAt.UTC(), in.EndsAt.UTC()
	e.AllDay, e.Location, e.Description, e.SourceURL = in.AllDay, in.Location, in.Description, in.SourceURL
	e.UpdatedAt = time.Now().UTC()
	return e, nil
}

func (m *memProvider) Delete(_ context.Context, id int) error {
	if _, ok := m.events[id]; !ok {
		return derr.ErrNotFound
	}
	delete(m.events, id)
	return nil
}

func (m *memProvider) Get(_ context.Context, id int) (*entity.CalendarEvent, error) {
	e, ok := m.events[id]
	if !ok {
		return nil, derr.ErrNotFound
	}
	return e, nil
}

func (m *memProvider) List(_ context.Context) ([]*entity.CalendarEvent, error) {
	out := make([]*entity.CalendarEvent, 0, len(m.events))
	for _, e := range m.events {
		out = append(out, e)
	}
	return out, nil
}

func (m *memProvider) ListByPeriod(_ context.Context, from, to time.Time) ([]*entity.CalendarEvent, error) {
	out := make([]*entity.CalendarEvent, 0)
	for _, e := range m.events {
		if e.StartsAt.Before(to) && e.EndsAt.After(from) {
			out = append(out, e)
		}
	}
	return out, nil
}

var _ calendarprovider.CalendarProvider = (*memProvider)(nil)

// memSettingsRepo は settings.Repository のインメモリ実装（handler テスト用）。
type memSettingsRepo struct {
	current *entity.AppSetting
}

func (m *memSettingsRepo) Get(_ context.Context) (*entity.AppSetting, error) {
	if m.current == nil {
		return nil, derr.ErrNotFound
	}
	return m.current, nil
}

func (m *memSettingsRepo) Upsert(_ context.Context, timezone string) (*entity.AppSetting, error) {
	m.current = &entity.AppSetting{ID: 1, Timezone: timezone, UpdatedAt: time.Now().UTC()}
	return m.current, nil
}
