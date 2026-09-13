package calendar_test

import (
	"context"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/calendarprovider"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider は CalendarProvider のインメモリ実装（テスト用）。
type fakeProvider struct {
	events map[int]*entity.CalendarEvent
	nextID int
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{events: map[int]*entity.CalendarEvent{}, nextID: 1}
}

func (f *fakeProvider) Create(_ context.Context, in calendarprovider.EventInput) (*entity.CalendarEvent, error) {
	e := &entity.CalendarEvent{
		ID: f.nextID, Title: in.Title, StartsAt: in.StartsAt.UTC(), EndsAt: in.EndsAt.UTC(),
		AllDay: in.AllDay, Location: in.Location, Description: in.Description, SourceURL: in.SourceURL,
	}
	f.events[f.nextID] = e
	f.nextID++
	return e, nil
}

func (f *fakeProvider) Update(_ context.Context, id int, in calendarprovider.EventInput) (*entity.CalendarEvent, error) {
	e, ok := f.events[id]
	if !ok {
		return nil, derr.ErrNotFound
	}
	e.Title, e.StartsAt, e.EndsAt = in.Title, in.StartsAt.UTC(), in.EndsAt.UTC()
	e.AllDay, e.Location, e.Description, e.SourceURL = in.AllDay, in.Location, in.Description, in.SourceURL
	return e, nil
}

func (f *fakeProvider) Delete(_ context.Context, id int) error {
	if _, ok := f.events[id]; !ok {
		return derr.ErrNotFound
	}
	delete(f.events, id)
	return nil
}

func (f *fakeProvider) Get(_ context.Context, id int) (*entity.CalendarEvent, error) {
	e, ok := f.events[id]
	if !ok {
		return nil, derr.ErrNotFound
	}
	return e, nil
}

func (f *fakeProvider) List(_ context.Context) ([]*entity.CalendarEvent, error) {
	out := make([]*entity.CalendarEvent, 0, len(f.events))
	for _, e := range f.events {
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeProvider) ListByPeriod(_ context.Context, from, to time.Time) ([]*entity.CalendarEvent, error) {
	out := make([]*entity.CalendarEvent, 0)
	for _, e := range f.events {
		if e.StartsAt.Before(to) && e.EndsAt.After(from) {
			out = append(out, e)
		}
	}
	return out, nil
}

var _ calendarprovider.CalendarProvider = (*fakeProvider)(nil)

func newUseCase() *calendar.UseCase {
	return calendar.New(newFakeProvider(), nil)
}

func validInput() calendar.EventInput {
	start := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	return calendar.EventInput{Title: "TGS", StartsAt: start, EndsAt: start.Add(time.Hour)}
}

func TestCreate_Success(t *testing.T) {
	uc := newUseCase()
	got, err := uc.Create(context.Background(), validInput())
	require.NoError(t, err)
	assert.NotZero(t, got.ID)
	assert.Equal(t, "TGS", got.Title)
}

func TestCreate_MissingTitle(t *testing.T) {
	uc := newUseCase()
	in := validInput()
	in.Title = ""
	_, err := uc.Create(context.Background(), in)
	require.Error(t, err)
	assert.True(t, derr.IsValidation(err))
}

func TestCreate_MissingStartsAt(t *testing.T) {
	uc := newUseCase()
	in := validInput()
	in.StartsAt = time.Time{}
	_, err := uc.Create(context.Background(), in)
	require.Error(t, err)
	assert.True(t, derr.IsValidation(err))
}

func TestCreate_EndBeforeStart(t *testing.T) {
	uc := newUseCase()
	in := validInput()
	in.EndsAt = in.StartsAt.Add(-time.Hour)
	_, err := uc.Create(context.Background(), in)
	require.Error(t, err)
	assert.True(t, derr.IsValidation(err))
}

func TestUpdate_Validation(t *testing.T) {
	uc := newUseCase()
	created, err := uc.Create(context.Background(), validInput())
	require.NoError(t, err)

	in := validInput()
	in.EndsAt = in.StartsAt.Add(-time.Hour)
	_, err = uc.Update(context.Background(), created.ID, in)
	assert.True(t, derr.IsValidation(err))
}

func TestGetUpdateDelete_NotFound(t *testing.T) {
	uc := newUseCase()
	_, err := uc.Get(context.Background(), 42)
	assert.ErrorIs(t, err, derr.ErrNotFound)
	err = uc.Delete(context.Background(), 42)
	assert.ErrorIs(t, err, derr.ErrNotFound)
	_, err = uc.Update(context.Background(), 42, validInput())
	assert.ErrorIs(t, err, derr.ErrNotFound)
}

func TestListByPeriod_OverlapAndBoundaries(t *testing.T) {
	uc := newUseCase()
	ctx := context.Background()
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	mk := func(title string, s, e time.Time) {
		_, err := uc.Create(ctx, calendar.EventInput{Title: title, StartsAt: s, EndsAt: e})
		require.NoError(t, err)
	}
	mk("inside", time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 15, 1, 0, 0, 0, time.UTC))
	mk("spanning", time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC))
	mk("before", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))
	// 境界: ends_at == from はヒットしない（ends_at > from が条件）
	mk("touch-from", time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), from)
	// 境界: starts_at == to はヒットしない（starts_at < to が条件）
	mk("touch-to", to, to.Add(time.Hour))

	got, err := uc.ListByPeriod(ctx, from, to)
	require.NoError(t, err)
	titles := make([]string, 0, len(got))
	for _, e := range got {
		titles = append(titles, e.Title)
	}
	assert.ElementsMatch(t, []string{"inside", "spanning"}, titles)
}

func TestListByPeriod_InvalidRange(t *testing.T) {
	uc := newUseCase()
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_, err := uc.ListByPeriod(context.Background(), from, to)
	assert.True(t, derr.IsValidation(err))
}
