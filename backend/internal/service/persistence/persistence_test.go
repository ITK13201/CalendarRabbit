package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
	"github.com/ITK13201/CalendarRabbit/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarEventRepository_CRUD(t *testing.T) {
	client := testsupport.NewClient(t)
	repo := persistence.NewCalendarEventRepository(client)
	ctx := context.Background()

	start := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	created, err := repo.Create(ctx, persistence.CalendarEventInput{
		Title:    "TGS 2026",
		StartsAt: start,
		EndsAt:   end,
		Location: "Makuhari Messe",
	})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, "TGS 2026", created.Title)

	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.True(t, start.Equal(got.StartsAt))

	updated, err := repo.Update(ctx, created.ID, persistence.CalendarEventInput{
		Title:    "TGS 2026 (updated)",
		StartsAt: start,
		EndsAt:   end,
		Location: "Chiba",
	})
	require.NoError(t, err)
	assert.Equal(t, "TGS 2026 (updated)", updated.Title)
	assert.Equal(t, "Chiba", updated.Location)

	require.NoError(t, repo.Delete(ctx, created.ID))
	_, err = repo.Get(ctx, created.ID)
	assert.ErrorIs(t, err, derr.ErrNotFound)
}

func TestCalendarEventRepository_NotFound(t *testing.T) {
	client := testsupport.NewClient(t)
	repo := persistence.NewCalendarEventRepository(client)
	ctx := context.Background()

	_, err := repo.Get(ctx, 999999)
	assert.ErrorIs(t, err, derr.ErrNotFound)
	err = repo.Delete(ctx, 999999)
	assert.ErrorIs(t, err, derr.ErrNotFound)
}

func TestCalendarEventRepository_ListByPeriod(t *testing.T) {
	client := testsupport.NewClient(t)
	repo := persistence.NewCalendarEventRepository(client)
	ctx := context.Background()

	// 対象月: 2026-09-01 .. 2026-10-01
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	mustCreate := func(title string, s, e time.Time) {
		_, err := repo.Create(ctx, persistence.CalendarEventInput{Title: title, StartsAt: s, EndsAt: e})
		require.NoError(t, err)
	}
	// 期間内
	mustCreate("in-month", time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC), time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC))
	// 期間前に開始し期間内へ継続する複数日イベント
	mustCreate("spanning", time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC), time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC))
	// 完全に期間外（前）
	mustCreate("before", time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC), time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
	// 完全に期間外（後）
	mustCreate("after", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC))

	got, err := repo.ListByPeriod(ctx, from, to)
	require.NoError(t, err)
	titles := make([]string, 0, len(got))
	for _, e := range got {
		titles = append(titles, e.Title)
	}
	assert.ElementsMatch(t, []string{"spanning", "in-month"}, titles)
	// 開始日時順（spanning が先）
	require.Len(t, got, 2)
	assert.Equal(t, "spanning", got[0].Title)
}

func TestAppSettingRepository_UpsertAndGet(t *testing.T) {
	client := testsupport.NewClient(t)
	repo := persistence.NewAppSettingRepository(client)
	ctx := context.Background()

	_, err := repo.Get(ctx)
	assert.ErrorIs(t, err, derr.ErrNotFound)

	created, err := repo.Upsert(ctx, "Asia/Tokyo")
	require.NoError(t, err)
	assert.Equal(t, "Asia/Tokyo", created.Timezone)

	updated, err := repo.Upsert(ctx, "UTC")
	require.NoError(t, err)
	assert.Equal(t, "UTC", updated.Timezone)
	assert.Equal(t, created.ID, updated.ID, "single-record: same row is updated")

	got, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "UTC", got.Timezone)
}

func TestConversationAndMessageRepository(t *testing.T) {
	client := testsupport.NewClient(t)
	convRepo := persistence.NewConversationRepository(client)
	msgRepo := persistence.NewMessageRepository(client)
	ctx := context.Background()

	conv, err := convRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	assert.NotZero(t, conv.ID)

	// GetOrCreate は同じスレッドを返す
	conv2, err := convRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	assert.Equal(t, conv.ID, conv2.ID)

	_, err = msgRepo.Create(ctx, conv.ID, entity.RoleUser, "TGSの予定を追加して")
	require.NoError(t, err)
	_, err = msgRepo.Create(ctx, conv.ID, entity.RoleAssistant, "TGS 2026 の予定案を作成しました")
	require.NoError(t, err)

	msgs, err := msgRepo.ListByConversation(ctx, conv.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, entity.RoleUser, msgs[0].Role)
	assert.Equal(t, entity.RoleAssistant, msgs[1].Role)
	assert.Equal(t, conv.ID, msgs[0].ConversationID)
}

func TestEventProposalRepository_LifecycleWithApproval(t *testing.T) {
	client := testsupport.NewClient(t)
	convRepo := persistence.NewConversationRepository(client)
	msgRepo := persistence.NewMessageRepository(client)
	propRepo := persistence.NewEventProposalRepository(client)
	eventRepo := persistence.NewCalendarEventRepository(client)
	ctx := context.Background()

	conv, err := convRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	msg, err := msgRepo.Create(ctx, conv.ID, entity.RoleAssistant, "予定案")
	require.NoError(t, err)

	start := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	prop, err := propRepo.Create(ctx, persistence.EventProposalInput{
		ConversationID: conv.ID,
		MessageID:      &msg.ID,
		Title:          "TGS 2026",
		StartsAt:       start,
		EndsAt:         start.Add(48 * time.Hour),
		Location:       "Makuhari",
	})
	require.NoError(t, err)
	assert.Equal(t, entity.ProposalStatusPending, prop.Status)
	assert.Equal(t, conv.ID, prop.ConversationID)
	require.NotNil(t, prop.MessageID)
	assert.Equal(t, msg.ID, *prop.MessageID)
	assert.Nil(t, prop.CalendarEventID)

	// 承認: CalendarEvent を作成し proposal に紐付ける
	event, err := eventRepo.Create(ctx, persistence.CalendarEventInput{
		Title:    prop.Title,
		StartsAt: prop.StartsAt,
		EndsAt:   prop.EndsAt,
		Location: prop.Location,
	})
	require.NoError(t, err)

	approved, err := propRepo.MarkApproved(ctx, prop.ID, event.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, entity.ProposalStatusApproved, approved.Status)
	require.NotNil(t, approved.CalendarEventID)
	assert.Equal(t, event.ID, *approved.CalendarEventID)

	// 会話単位の一覧取得
	list, err := propRepo.ListByConversation(ctx, conv.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, entity.ProposalStatusApproved, list[0].Status)
}

func TestWithTx_RollbackOnError(t *testing.T) {
	client := testsupport.NewClient(t)
	ctx := context.Background()

	sentinel := assert.AnError
	err := persistence.WithTx(ctx, client, func(txClient *ent.Client) error {
		_, cerr := persistence.NewConversationRepository(txClient).GetOrCreate(ctx)
		require.NoError(t, cerr)
		return sentinel // ロールバックさせる
	})
	assert.ErrorIs(t, err, sentinel)

	// ロールバックされたので会話は存在しない
	count, err := persistence.NewCalendarEventRepository(client).List(ctx)
	require.NoError(t, err)
	assert.Empty(t, count)
}
