package calendarprovider_test

import (
	"context"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/calendarprovider"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
	"github.com/ITK13201/CalendarRabbit/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProvider(t *testing.T) calendarprovider.CalendarProvider {
	client := testsupport.NewClient(t)
	return calendarprovider.NewDBProvider(persistence.NewCalendarEventRepository(client))
}

func TestDBProvider_CRUD(t *testing.T) {
	p := newProvider(t)
	ctx := context.Background()

	start := time.Date(2026, 9, 26, 1, 0, 0, 0, time.UTC)
	created, err := p.Create(ctx, calendarprovider.EventInput{
		Title:    "Event A",
		StartsAt: start,
		EndsAt:   start.Add(time.Hour),
	})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)

	got, err := p.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Event A", got.Title)

	updated, err := p.Update(ctx, created.ID, calendarprovider.EventInput{
		Title:    "Event B",
		StartsAt: start,
		EndsAt:   start.Add(time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "Event B", updated.Title)

	require.NoError(t, p.Delete(ctx, created.ID))
	_, err = p.Get(ctx, created.ID)
	assert.ErrorIs(t, err, derr.ErrNotFound)
}

func TestDBProvider_ListByPeriod(t *testing.T) {
	p := newProvider(t)
	ctx := context.Background()

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	_, err := p.Create(ctx, calendarprovider.EventInput{
		Title: "inside", StartsAt: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	_, err = p.Create(ctx, calendarprovider.EventInput{
		Title: "outside", StartsAt: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	got, err := p.ListByPeriod(ctx, from, to)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "inside", got[0].Title)
}
