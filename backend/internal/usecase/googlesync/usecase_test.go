package googlesync_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/google"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/googlesync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeConnStore struct {
	conn *entity.GoogleConnection
}

func (f *fakeConnStore) Get(context.Context) (*entity.GoogleConnection, error) {
	if f.conn == nil {
		return nil, derr.ErrNotFound
	}
	c := *f.conn
	return &c, nil
}

func (f *fakeConnStore) Save(_ context.Context, in persistence.GoogleConnectionInput) (*entity.GoogleConnection, error) {
	f.conn = &entity.GoogleConnection{
		ID: 1, RefreshToken: in.RefreshToken, AccessToken: in.AccessToken,
		TokenExpiry: in.TokenExpiry, CalendarID: in.CalendarID, Connected: in.Connected,
	}
	c := *f.conn
	return &c, nil
}

func (f *fakeConnStore) MarkDisconnected(_ context.Context, clearCalendar bool) error {
	if f.conn != nil {
		f.conn.Connected = false
		f.conn.RefreshToken = ""
		if clearCalendar {
			f.conn.CalendarID = ""
		}
	}
	return nil
}

func (f *fakeConnStore) Clear(context.Context) error {
	f.conn = nil
	return nil
}

type fakeEventStore struct {
	events map[int]*entity.CalendarEvent
}

func newFakeEventStore(evs ...*entity.CalendarEvent) *fakeEventStore {
	m := map[int]*entity.CalendarEvent{}
	for _, e := range evs {
		m[e.ID] = e
	}
	return &fakeEventStore{events: m}
}

func (f *fakeEventStore) List(context.Context) ([]*entity.CalendarEvent, error) {
	out := make([]*entity.CalendarEvent, 0, len(f.events))
	for _, e := range f.events {
		c := *e
		out = append(out, &c)
	}
	return out, nil
}

func (f *fakeEventStore) ListPending(context.Context) ([]*entity.CalendarEvent, error) {
	out := make([]*entity.CalendarEvent, 0)
	for _, e := range f.events {
		if e.SyncPending {
			c := *e
			out = append(out, &c)
		}
	}
	return out, nil
}

func (f *fakeEventStore) SetGoogleMapping(_ context.Context, id int, gid string, pending bool) error {
	e, ok := f.events[id]
	if !ok {
		return derr.ErrNotFound
	}
	e.GoogleEventID = gid
	e.SyncPending = pending
	return nil
}

func (f *fakeEventStore) ClearAllGoogleMappings(context.Context) error {
	for _, e := range f.events {
		e.GoogleEventID = ""
		e.SyncPending = false
	}
	return nil
}

func (f *fakeEventStore) CountPending(context.Context) (int, error) {
	n := 0
	for _, e := range f.events {
		if e.SyncPending {
			n++
		}
	}
	return n, nil
}

type fakeSettings struct{}

func (fakeSettings) Get(context.Context) (*entity.AppSetting, error) {
	return &entity.AppSetting{Timezone: "Asia/Tokyo"}, nil
}

type fakeClient struct {
	createCalendarCalls int
	deleteCalendarCalls int
	createEventCalls    int
	createEventErr      error
	updateEventErr      error
	nextID              int
}

func (c *fakeClient) CreateCalendar(context.Context, string) (string, error) {
	c.createCalendarCalls++
	return "cal-created", nil
}
func (c *fakeClient) DeleteCalendar(context.Context, string) error {
	c.deleteCalendarCalls++
	return nil
}
func (c *fakeClient) CreateEvent(context.Context, string, google.EventData) (string, error) {
	c.createEventCalls++
	if c.createEventErr != nil {
		return "", c.createEventErr
	}
	c.nextID++
	return fmt.Sprintf("gevt-%d", c.nextID), nil
}
func (c *fakeClient) UpdateEvent(context.Context, string, string, google.EventData) error {
	return c.updateEventErr
}
func (c *fakeClient) DeleteEvent(context.Context, string, string) error { return nil }

type fakeFactory struct {
	client    *fakeClient
	token     *google.Token
	clientErr error
}

func (f *fakeFactory) AuthCodeURL(state string) string {
	return "https://accounts.google.com/o/oauth2/auth?state=" + state
}
func (f *fakeFactory) Exchange(context.Context, string) (*google.Token, error) {
	return f.token, nil
}
func (f *fakeFactory) Client(context.Context, string) (google.CalendarClient, error) {
	return f.client, f.clientErr
}

func newUC(t *testing.T, conns *fakeConnStore, events *fakeEventStore, factory *fakeFactory) *googlesync.UseCase {
	t.Helper()
	return googlesync.New(conns, events, fakeSettings{}, factory, nil)
}

func tokenWithRefresh() *google.Token {
	return &google.Token{RefreshToken: "rt", AccessToken: "at"}
}

// --- 5.1 ---

func TestConnect_CreatesDedicatedCalendarWhenMissing(t *testing.T) {
	conns := &fakeConnStore{}
	events := newFakeEventStore()
	client := &fakeClient{}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	require.NoError(t, uc.Connect(context.Background(), "auth-code"))
	assert.Equal(t, 1, client.createCalendarCalls)
	require.NotNil(t, conns.conn)
	assert.True(t, conns.conn.Connected)
	assert.Equal(t, "cal-created", conns.conn.CalendarID)
	assert.Equal(t, "rt", conns.conn.RefreshToken)
}

func TestConnect_ReusesExistingCalendarID(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-existing", RefreshToken: "old"}}
	events := newFakeEventStore()
	client := &fakeClient{}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	require.NoError(t, uc.Connect(context.Background(), "auth-code"))
	assert.Equal(t, 0, client.createCalendarCalls) // 重複作成しない
	assert.Equal(t, "cal-existing", conns.conn.CalendarID)
}

// --- 5.2 backfill ---

func TestConnect_BackfillsExistingEvents(t *testing.T) {
	conns := &fakeConnStore{}
	events := newFakeEventStore(
		&entity.CalendarEvent{ID: 1, Title: "A"},
		&entity.CalendarEvent{ID: 2, Title: "B"},
	)
	client := &fakeClient{}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	require.NoError(t, uc.Connect(context.Background(), "auth-code"))
	assert.NotEmpty(t, events.events[1].GoogleEventID)
	assert.NotEmpty(t, events.events[2].GoogleEventID)
	assert.False(t, events.events[1].SyncPending)
	assert.False(t, events.events[2].SyncPending)
}

func TestConnect_BackfillWithNoEvents(t *testing.T) {
	conns := &fakeConnStore{}
	events := newFakeEventStore()
	client := &fakeClient{}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	require.NoError(t, uc.Connect(context.Background(), "auth-code"))
	assert.True(t, conns.conn.Connected) // 何も作らず連携は有効。
}

// --- 5.3 disconnect ---

func TestDisconnect_DeleteCalendar(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", RefreshToken: "rt", Connected: true}}
	events := newFakeEventStore(&entity.CalendarEvent{ID: 1, GoogleEventID: "g1"})
	client := &fakeClient{}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	require.NoError(t, uc.Disconnect(context.Background(), true))
	assert.Equal(t, 1, client.deleteCalendarCalls)
	assert.Nil(t, conns.conn) // ローカル破棄
	assert.Empty(t, events.events[1].GoogleEventID)
}

func TestDisconnect_KeepCalendar(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", RefreshToken: "rt", Connected: true}}
	events := newFakeEventStore(&entity.CalendarEvent{ID: 1, GoogleEventID: "g1"})
	client := &fakeClient{}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	require.NoError(t, uc.Disconnect(context.Background(), false))
	assert.Equal(t, 0, client.deleteCalendarCalls) // 残す
	assert.Nil(t, conns.conn)                      // ローカルは破棄
}

func TestDisconnect_WhenNotConnected(t *testing.T) {
	conns := &fakeConnStore{}
	uc := newUC(t, conns, newFakeEventStore(), &fakeFactory{client: &fakeClient{}})
	require.NoError(t, uc.Disconnect(context.Background(), true)) // 何もせず正常終了
}

// --- 5.4 resync ---

func TestResync_ResendsPending(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", RefreshToken: "rt", Connected: true}}
	events := newFakeEventStore(
		&entity.CalendarEvent{ID: 1, Title: "A", SyncPending: true},
		&entity.CalendarEvent{ID: 2, Title: "B", SyncPending: false, GoogleEventID: "g2"},
	)
	client := &fakeClient{}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	remaining, err := uc.Resync(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
	assert.False(t, events.events[1].SyncPending)
	assert.NotEmpty(t, events.events[1].GoogleEventID)
}

func TestResync_NoPending(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", RefreshToken: "rt", Connected: true}}
	events := newFakeEventStore(&entity.CalendarEvent{ID: 1, GoogleEventID: "g1"})
	uc := newUC(t, conns, events, &fakeFactory{client: &fakeClient{}, token: tokenWithRefresh()})

	remaining, err := uc.Resync(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
}

func TestResync_PartialFailureKeepsPending(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", RefreshToken: "rt", Connected: true}}
	events := newFakeEventStore(&entity.CalendarEvent{ID: 1, Title: "A", SyncPending: true})
	client := &fakeClient{createEventErr: errors.New("google down")}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	remaining, err := uc.Resync(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, remaining) // 失敗分は未同期のまま
	assert.True(t, events.events[1].SyncPending)
}

func TestResync_NotConnected(t *testing.T) {
	uc := newUC(t, &fakeConnStore{}, newFakeEventStore(), &fakeFactory{client: &fakeClient{}})
	_, err := uc.Resync(context.Background())
	assert.ErrorIs(t, err, googlesync.ErrNotConnected)
}

// A: 再同期中にトークン失効を検知したら未連携化し ErrReconnectRequired を返す（calendar_id 保持）。
func TestResync_TokenRevoked_RequiresReconnect(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", RefreshToken: "rt", Connected: true}}
	events := newFakeEventStore(&entity.CalendarEvent{ID: 1, Title: "A", SyncPending: true})
	client := &fakeClient{createEventErr: errors.Join(google.ErrTokenRevoked, errors.New("401"))}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	_, err := uc.Resync(context.Background())
	assert.ErrorIs(t, err, googlesync.ErrReconnectRequired)
	assert.False(t, conns.conn.Connected)
	assert.Equal(t, "cal-1", conns.conn.CalendarID) // 保持
}

// B: 再同期中にカレンダー消失（404）を検知したら未連携化し calendar_id とマッピングを破棄する。
func TestResync_CalendarNotFound_ClearsAndRequiresReconnect(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", RefreshToken: "rt", Connected: true}}
	events := newFakeEventStore(&entity.CalendarEvent{ID: 1, Title: "A", SyncPending: true})
	client := &fakeClient{createEventErr: errors.Join(google.ErrRemoteNotFound, errors.New("404"))}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	_, err := uc.Resync(context.Background())
	assert.ErrorIs(t, err, googlesync.ErrReconnectRequired)
	assert.False(t, conns.conn.Connected)
	assert.Empty(t, conns.conn.CalendarID)
	assert.Empty(t, events.events[1].GoogleEventID) // マッピング破棄
}

// B: イベントが Google 側で手動削除（Update 404）されたら再作成する。
func TestResync_EventDeleted_Recreates(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", RefreshToken: "rt", Connected: true}}
	events := newFakeEventStore(&entity.CalendarEvent{ID: 1, Title: "A", SyncPending: true, GoogleEventID: "old"})
	client := &fakeClient{updateEventErr: errors.Join(google.ErrRemoteNotFound, errors.New("404"))}
	uc := newUC(t, conns, events, &fakeFactory{client: client, token: tokenWithRefresh()})

	remaining, err := uc.Resync(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
	assert.Equal(t, 1, client.createEventCalls) // 再作成された
	assert.NotEqual(t, "old", events.events[1].GoogleEventID)
	assert.False(t, events.events[1].SyncPending)
}

// status: 失効で未連携相当に落ちた状態は NeedsReconnect=true を返す。
func TestStatus_NeedsReconnect(t *testing.T) {
	// 連携履歴あり（レコード残存）だが Connected=false。
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", Connected: false}}
	uc := newUC(t, conns, newFakeEventStore(), &fakeFactory{client: &fakeClient{}})

	st, err := uc.Status(context.Background())
	require.NoError(t, err)
	assert.False(t, st.Connected)
	assert.True(t, st.NeedsReconnect)
}

// --- status ---

func TestStatus_ConnectedWithPending(t *testing.T) {
	conns := &fakeConnStore{conn: &entity.GoogleConnection{CalendarID: "cal-1", Connected: true}}
	events := newFakeEventStore(&entity.CalendarEvent{ID: 1, SyncPending: true})
	uc := newUC(t, conns, events, &fakeFactory{client: &fakeClient{}})

	st, err := uc.Status(context.Background())
	require.NoError(t, err)
	assert.True(t, st.Connected)
	assert.Equal(t, 1, st.PendingCount)
	assert.Equal(t, "cal-1", st.CalendarID)
}

func TestStatus_NotConnected(t *testing.T) {
	uc := newUC(t, &fakeConnStore{}, newFakeEventStore(), &fakeFactory{client: &fakeClient{}})
	st, err := uc.Status(context.Background())
	require.NoError(t, err)
	assert.False(t, st.Connected)
}
