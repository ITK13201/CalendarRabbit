package calendarprovider_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/calendarprovider"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/google"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

// fakeStore は CalendarProvider（内側 DB）と MappingWriter を兼ねるインメモリ実装。
type fakeStore struct {
	events map[int]*entity.CalendarEvent
	nextID int
}

func newFakeStore() *fakeStore {
	return &fakeStore{events: map[int]*entity.CalendarEvent{}, nextID: 1}
}

func (s *fakeStore) Create(_ context.Context, in calendarprovider.EventInput) (*entity.CalendarEvent, error) {
	e := &entity.CalendarEvent{
		ID: s.nextID, Title: in.Title, StartsAt: in.StartsAt.UTC(), EndsAt: in.EndsAt.UTC(),
		AllDay: in.AllDay, Location: in.Location, Description: in.Description, SourceURL: in.SourceURL,
	}
	s.events[s.nextID] = e
	s.nextID++
	return clone(e), nil
}

func (s *fakeStore) Update(_ context.Context, id int, in calendarprovider.EventInput) (*entity.CalendarEvent, error) {
	e, ok := s.events[id]
	if !ok {
		return nil, derr.ErrNotFound
	}
	e.Title, e.StartsAt, e.EndsAt = in.Title, in.StartsAt.UTC(), in.EndsAt.UTC()
	e.AllDay, e.Location, e.Description, e.SourceURL = in.AllDay, in.Location, in.Description, in.SourceURL
	return clone(e), nil
}

func (s *fakeStore) Delete(_ context.Context, id int) error {
	if _, ok := s.events[id]; !ok {
		return derr.ErrNotFound
	}
	delete(s.events, id)
	return nil
}

func (s *fakeStore) Get(_ context.Context, id int) (*entity.CalendarEvent, error) {
	e, ok := s.events[id]
	if !ok {
		return nil, derr.ErrNotFound
	}
	return clone(e), nil
}

func (s *fakeStore) List(_ context.Context) ([]*entity.CalendarEvent, error) {
	out := make([]*entity.CalendarEvent, 0, len(s.events))
	for _, e := range s.events {
		out = append(out, clone(e))
	}
	return out, nil
}

func (s *fakeStore) ListByPeriod(_ context.Context, _, _ time.Time) ([]*entity.CalendarEvent, error) {
	return s.List(context.Background())
}

func (s *fakeStore) SetGoogleMapping(_ context.Context, id int, googleEventID string, syncPending bool) error {
	e, ok := s.events[id]
	if !ok {
		return derr.ErrNotFound
	}
	e.GoogleEventID = googleEventID
	e.SyncPending = syncPending
	return nil
}

func (s *fakeStore) ClearAllGoogleMappings(context.Context) error {
	for _, e := range s.events {
		e.GoogleEventID = ""
		e.SyncPending = false
	}
	return nil
}

func clone(e *entity.CalendarEvent) *entity.CalendarEvent {
	c := *e
	return &c
}

// fakeGoogleClient は google.CalendarClient のフェイク。呼び出しを記録し、任意で失敗させる。
type fakeGoogleClient struct {
	createErr, updateErr, deleteErr error
	lastCreate                      google.EventData
	created, updated                []string
	deleted                         []string
	nextID                          int
}

func (f *fakeGoogleClient) CreateCalendar(_ context.Context, _ string) (string, error) {
	return "cal-new", nil
}
func (f *fakeGoogleClient) DeleteCalendar(_ context.Context, _ string) error { return nil }

func (f *fakeGoogleClient) CreateEvent(_ context.Context, _ string, ev google.EventData) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	f.lastCreate = ev
	f.nextID++
	id := fmt.Sprintf("gevt-%d", f.nextID)
	f.created = append(f.created, id)
	return id, nil
}

func (f *fakeGoogleClient) UpdateEvent(_ context.Context, _, eventID string, _ google.EventData) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = append(f.updated, eventID)
	return nil
}

func (f *fakeGoogleClient) DeleteEvent(_ context.Context, _, eventID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, eventID)
	return nil
}

type fakeFactory struct {
	client    google.CalendarClient
	clientErr error
}

func (f *fakeFactory) AuthCodeURL(string) string { return "" }
func (f *fakeFactory) Exchange(context.Context, string) (*google.Token, error) {
	return nil, nil
}
func (f *fakeFactory) Client(context.Context, string) (google.CalendarClient, error) {
	return f.client, f.clientErr
}

type fakeConns struct {
	conn                *entity.GoogleConnection
	disconnectedCleared bool
	disconnectedKept    bool
}

func (f *fakeConns) Get(context.Context) (*entity.GoogleConnection, error) {
	if f.conn == nil {
		return nil, derr.ErrNotFound
	}
	return f.conn, nil
}

func (f *fakeConns) MarkDisconnected(_ context.Context, clearCalendar bool) error {
	if f.conn != nil {
		f.conn.Connected = false
		if clearCalendar {
			f.conn.CalendarID = ""
		}
	}
	if clearCalendar {
		f.disconnectedCleared = true
	} else {
		f.disconnectedKept = true
	}
	return nil
}

type fakeSettings struct{ tz string }

func (f *fakeSettings) Get(context.Context) (*entity.AppSetting, error) {
	return &entity.AppSetting{Timezone: f.tz}, nil
}

// --- helpers ---

func connectedConn() *entity.GoogleConnection {
	return &entity.GoogleConnection{RefreshToken: "rt", CalendarID: "cal-1", Connected: true}
}

func newMirror(t *testing.T, conn *entity.GoogleConnection, client google.CalendarClient) (*calendarprovider.MirroringProvider, *fakeStore, *fakeConns) {
	t.Helper()
	store := newFakeStore()
	conns := &fakeConns{conn: conn}
	inner := calendarprovider.CalendarProvider(store)
	mp := calendarprovider.NewMirroringProvider(
		inner,
		conns,
		&fakeSettings{tz: "Asia/Tokyo"},
		store,
		&fakeFactory{client: client},
		nil,
	)
	return mp, store, conns
}

func sampleInput() calendarprovider.EventInput {
	start := time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC)
	return calendarprovider.EventInput{Title: "Meeting", StartsAt: start, EndsAt: start.Add(time.Hour)}
}

// --- tests ---

// 4.1 インターフェース充足（コンパイル時アサーション）。
var _ calendarprovider.CalendarProvider = (*calendarprovider.MirroringProvider)(nil)

// 4.2 連携無効時は Google を呼ばず DB のみで完結する。
func TestMirroring_Disconnected_NoGoogleCall(t *testing.T) {
	client := &fakeGoogleClient{}
	mp, store, _ := newMirror(t, nil, client) // conn nil → 未連携

	ev, err := mp.Create(context.Background(), sampleInput())
	require.NoError(t, err)
	assert.Empty(t, ev.GoogleEventID)
	assert.False(t, ev.SyncPending)
	assert.Empty(t, client.created)
	assert.False(t, store.events[ev.ID].SyncPending)
}

// 4.2 連携有効時の Create は Google へ作成しマッピングを保存する。
func TestMirroring_Connected_CreateMirrors(t *testing.T) {
	client := &fakeGoogleClient{}
	mp, store, _ := newMirror(t, connectedConn(), client)

	ev, err := mp.Create(context.Background(), sampleInput())
	require.NoError(t, err)
	require.Len(t, client.created, 1)
	assert.Equal(t, client.created[0], ev.GoogleEventID)
	assert.False(t, ev.SyncPending)
	assert.Equal(t, client.created[0], store.events[ev.ID].GoogleEventID)
	assert.False(t, store.events[ev.ID].SyncPending)
}

// 4.2 連携有効時の Update は既存マッピングがあれば Google を更新する。
func TestMirroring_Connected_UpdateMirrors(t *testing.T) {
	client := &fakeGoogleClient{}
	mp, store, _ := newMirror(t, connectedConn(), client)

	created, err := mp.Create(context.Background(), sampleInput())
	require.NoError(t, err)

	in := sampleInput()
	in.Title = "Updated"
	updated, err := mp.Update(context.Background(), created.ID, in)
	require.NoError(t, err)
	require.Len(t, client.updated, 1)
	assert.Equal(t, created.GoogleEventID, client.updated[0])
	assert.False(t, updated.SyncPending)
	assert.False(t, store.events[created.ID].SyncPending)
}

// 4.2 連携有効時の Delete は Google の対応イベントを削除する。
func TestMirroring_Connected_DeleteMirrors(t *testing.T) {
	client := &fakeGoogleClient{}
	mp, _, _ := newMirror(t, connectedConn(), client)

	created, err := mp.Create(context.Background(), sampleInput())
	require.NoError(t, err)

	require.NoError(t, mp.Delete(context.Background(), created.ID))
	require.Len(t, client.deleted, 1)
	assert.Equal(t, created.GoogleEventID, client.deleted[0])
}

// 4.3 同期失敗時は DB 操作を成功で返しつつ sync_pending=true を立てる。
func TestMirroring_CreateFailure_MarksPending(t *testing.T) {
	client := &fakeGoogleClient{createErr: errors.New("google down")}
	mp, store, _ := newMirror(t, connectedConn(), client)

	ev, err := mp.Create(context.Background(), sampleInput())
	require.NoError(t, err) // DB 操作は成功
	assert.True(t, ev.SyncPending)
	assert.Empty(t, ev.GoogleEventID)
	assert.True(t, store.events[ev.ID].SyncPending)
}

// 4.3 失敗後、未同期イベントは次回 Update で再送され pending が解消する。
func TestMirroring_PendingResentOnNextUpdate(t *testing.T) {
	client := &fakeGoogleClient{createErr: errors.New("google down")}
	mp, store, _ := newMirror(t, connectedConn(), client)

	ev, err := mp.Create(context.Background(), sampleInput())
	require.NoError(t, err)
	require.True(t, store.events[ev.ID].SyncPending)

	// Google 復旧 → 次の Update で作成し直され pending 解消。
	client.createErr = nil
	in := sampleInput()
	in.Title = "Recovered"
	updated, err := mp.Update(context.Background(), ev.ID, in)
	require.NoError(t, err)
	assert.False(t, updated.SyncPending)
	assert.NotEmpty(t, updated.GoogleEventID)
	assert.False(t, store.events[ev.ID].SyncPending)
}

// 4.4 終日/タイムゾーンのマッピングが Google 入力へ反映される。
func TestMirroring_MapsAllDayAndTimezone(t *testing.T) {
	client := &fakeGoogleClient{}
	mp, _, _ := newMirror(t, connectedConn(), client)

	in := sampleInput()
	in.AllDay = true
	_, err := mp.Create(context.Background(), in)
	require.NoError(t, err)
	assert.True(t, client.lastCreate.AllDay)
	assert.Equal(t, "Asia/Tokyo", client.lastCreate.TimeZone)
}

// A: トークン失効時は未同期にしつつ連携を未連携相当へ落とす（calendar_id は保持）。
func TestMirroring_TokenRevoked_MarksDisconnected(t *testing.T) {
	client := &fakeGoogleClient{createErr: errors.Join(google.ErrTokenRevoked, errors.New("401"))}
	mp, store, conns := newMirror(t, connectedConn(), client)

	ev, err := mp.Create(context.Background(), sampleInput())
	require.NoError(t, err) // DB は成功
	assert.True(t, store.events[ev.ID].SyncPending)
	assert.True(t, conns.disconnectedKept)      // MarkDisconnected(clearCalendar=false)
	assert.False(t, conns.disconnectedCleared)
	assert.False(t, conns.conn.Connected)
	assert.Equal(t, "cal-1", conns.conn.CalendarID) // calendar_id は保持（再連携で再利用）
}

// B: イベントが Google 側で手動削除されると、Update の 404 を検知して再作成する。
func TestMirroring_EventDeletedInGoogle_Recreates(t *testing.T) {
	client := &fakeGoogleClient{}
	mp, store, _ := newMirror(t, connectedConn(), client)

	created, err := mp.Create(context.Background(), sampleInput())
	require.NoError(t, err)
	require.Len(t, client.created, 1)

	// 以後 Update は 404 を返す（イベントが手動削除された）。
	client.updateErr = errors.Join(google.ErrRemoteNotFound, errors.New("404"))
	in := sampleInput()
	in.Title = "Edited"
	updated, err := mp.Update(context.Background(), created.ID, in)
	require.NoError(t, err)
	// 再作成され、新しい google_event_id が付与され pending にならない。
	assert.Len(t, client.created, 2)
	assert.Equal(t, client.created[1], updated.GoogleEventID)
	assert.False(t, store.events[created.ID].SyncPending)
}

// B: 専用カレンダーが手動削除されると、Create の 404 を検知して未連携化しマッピングを破棄する。
func TestMirroring_CalendarDeletedInGoogle_Disconnects(t *testing.T) {
	client := &fakeGoogleClient{createErr: errors.Join(google.ErrRemoteNotFound, errors.New("404"))}
	mp, store, conns := newMirror(t, connectedConn(), client)

	ev, err := mp.Create(context.Background(), sampleInput())
	require.NoError(t, err)
	assert.True(t, conns.disconnectedCleared) // MarkDisconnected(clearCalendar=true)
	assert.False(t, conns.conn.Connected)
	assert.Empty(t, conns.conn.CalendarID)
	// マッピングは破棄済み（再連携で作り直す）。
	assert.Empty(t, store.events[ev.ID].GoogleEventID)
}
