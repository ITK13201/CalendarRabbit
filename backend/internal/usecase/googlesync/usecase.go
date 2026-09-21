// Package googlesync は Google Calendar 連携のライフサイクル（OAuth 連携確立・
// 初回バックフィル・連携解除・未同期の手動再同期・連携状態取得）のユースケースを提供する。
package googlesync

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/google"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
)

// ErrNotConnected は連携が有効でない状態で連携前提の操作を要求したときに返す。
var ErrNotConnected = errors.New("googlesync: not connected")

// ErrNoRefreshToken は認可コード交換で refresh token が得られず、既存トークンも無いときに返す。
var ErrNoRefreshToken = errors.New("googlesync: no refresh token returned; reconnect with consent")

// ErrReconnectRequired はトークン失効や専用カレンダー消失を検知し、連携を未連携相当へ
// 落としたときに返す。呼び出し側は再連携を促す。
var ErrReconnectRequired = errors.New("googlesync: reconnect required (token revoked or calendar removed)")

// ConnectionStore は連携状態の永続化を抽象化する。
type ConnectionStore interface {
	Get(ctx context.Context) (*entity.GoogleConnection, error)
	Save(ctx context.Context, in persistence.GoogleConnectionInput) (*entity.GoogleConnection, error)
	MarkDisconnected(ctx context.Context, clearCalendar bool) error
	Clear(ctx context.Context) error
}

// EventStore はミラー対象イベントの参照とマッピング更新を抽象化する。
type EventStore interface {
	List(ctx context.Context) ([]*entity.CalendarEvent, error)
	ListPending(ctx context.Context) ([]*entity.CalendarEvent, error)
	SetGoogleMapping(ctx context.Context, id int, googleEventID string, syncPending bool) error
	ClearAllGoogleMappings(ctx context.Context) error
	CountPending(ctx context.Context) (int, error)
}

// SettingStore はタイムゾーン取得のための設定リーダ。
type SettingStore interface {
	Get(ctx context.Context) (*entity.AppSetting, error)
}

// UseCase は Google 連携ライフサイクルのユースケース。
type UseCase struct {
	conns    ConnectionStore
	events   EventStore
	settings SettingStore
	factory  google.ClientFactory
	logger   *slog.Logger
}

// New は UseCase を生成する。
func New(conns ConnectionStore, events EventStore, settings SettingStore, factory google.ClientFactory, logger *slog.Logger) *UseCase {
	if logger == nil {
		logger = slog.Default()
	}
	return &UseCase{conns: conns, events: events, settings: settings, factory: factory, logger: logger}
}

// Status は連携状態のスナップショット。
type Status struct {
	Connected  bool
	CalendarID string
	// NeedsReconnect はトークン失効やカレンダー消失で未連携相当へ落ちており、
	// 再連携が必要な状態を示す（連携履歴はあるが Connected=false）。
	NeedsReconnect bool
	PendingCount   int
}

// AuthURL は同意画面の認可 URL を返す（CSRF 対策の state を付与する）。
func (u *UseCase) AuthURL(state string) string {
	return u.factory.AuthCodeURL(state)
}

// Status は現在の連携状態と未同期件数を返す。
func (u *UseCase) Status(ctx context.Context) (res *Status, err error) {
	defer logging.Trace(ctx, u.logger, "googlesync.UseCase.Status", nil, &res, &err)()

	pending, err := u.events.CountPending(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := u.conns.Get(ctx)
	if err != nil {
		if errors.Is(err, derr.ErrNotFound) {
			return &Status{Connected: false, PendingCount: pending}, nil
		}
		return nil, err
	}
	// レコードが残っているのに未接続 ＝ 失効/カレンダー消失で落とされた状態（再連携が必要）。
	return &Status{
		Connected:      conn.Connected,
		CalendarID:     conn.CalendarID,
		NeedsReconnect: !conn.Connected,
		PendingCount:   pending,
	}, nil
}

// Connect は認可コードをトークンへ交換し、連携状態を保存する。
// 専用カレンダー未保持なら「CalendarRabbit」を1件だけ作成し、既存 DB イベントをバックフィルする。
func (u *UseCase) Connect(ctx context.Context, code string) (err error) {
	defer logging.Trace(ctx, u.logger, "googlesync.UseCase.Connect", nil, nil, &err)()

	tok, err := u.factory.Exchange(ctx, code)
	if err != nil {
		return err
	}

	existing, gerr := u.conns.Get(ctx)
	if gerr != nil && !errors.Is(gerr, derr.ErrNotFound) {
		return gerr
	}

	refreshToken := tok.RefreshToken
	if refreshToken == "" {
		// 再同意でも refresh token が返らない場合、既存トークンがあれば再利用する。
		if existing != nil && existing.RefreshToken != "" {
			refreshToken = existing.RefreshToken
		} else {
			return ErrNoRefreshToken
		}
	}

	client, err := u.factory.Client(ctx, refreshToken)
	if err != nil {
		return err
	}

	// 専用カレンダーの calendarId を未保持なら1件だけ作成する（重複作成しない）。
	calendarID := ""
	if existing != nil {
		calendarID = existing.CalendarID
	}
	if calendarID == "" {
		calendarID, err = client.CreateCalendar(ctx, google.CalendarName)
		if err != nil {
			return err
		}
	}

	if _, err = u.conns.Save(ctx, persistence.GoogleConnectionInput{
		RefreshToken: refreshToken,
		AccessToken:  tok.AccessToken,
		TokenExpiry:  tok.Expiry,
		CalendarID:   calendarID,
		Connected:    true,
	}); err != nil {
		return err
	}

	// 初回連携時のバックフィル: DB 既存イベントを専用カレンダーへ作成する。
	all, err := u.events.List(ctx)
	if err != nil {
		return err
	}
	if rerr := u.resend(ctx, client, calendarID, all); rerr != nil {
		return u.handleResendFailure(ctx, rerr)
	}
	return nil
}

// Resync は未同期（sync_pending）イベントを専用カレンダーへ一括再送し、残りの未同期件数を返す。
func (u *UseCase) Resync(ctx context.Context) (remaining int, err error) {
	defer logging.Trace(ctx, u.logger, "googlesync.UseCase.Resync", nil, &remaining, &err)()

	conn, err := u.conns.Get(ctx)
	if err != nil {
		if errors.Is(err, derr.ErrNotFound) {
			return 0, ErrNotConnected
		}
		return 0, err
	}
	if !conn.Connected || conn.CalendarID == "" || conn.RefreshToken == "" {
		return 0, ErrNotConnected
	}

	pending, err := u.events.ListPending(ctx)
	if err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil // 未同期0件でも正常完了。
	}

	client, err := u.factory.Client(ctx, conn.RefreshToken)
	if err != nil {
		return 0, err
	}
	if rerr := u.resend(ctx, client, conn.CalendarID, pending); rerr != nil {
		if herr := u.handleResendFailure(ctx, rerr); herr != nil {
			return 0, herr // ErrReconnectRequired（失効/カレンダー消失）
		}
	}
	return u.events.CountPending(ctx)
}

// Disconnect は連携を解除する。deleteCalendar=true なら専用カレンダーごと削除する。
// いずれの場合もローカルの連携状態（トークン・calendarId・イベントマッピング）を破棄する。
func (u *UseCase) Disconnect(ctx context.Context, deleteCalendar bool) (err error) {
	defer logging.Trace(ctx, u.logger, "googlesync.UseCase.Disconnect", logging.Args{"deleteCalendar": deleteCalendar}, nil, &err)()

	conn, err := u.conns.Get(ctx)
	if err != nil {
		if errors.Is(err, derr.ErrNotFound) {
			return nil // 既に未連携。
		}
		return err
	}

	if deleteCalendar && conn.CalendarID != "" && conn.RefreshToken != "" {
		client, cerr := u.factory.Client(ctx, conn.RefreshToken)
		if cerr != nil {
			// クライアント生成に失敗しても、ローカル状態は必ず破棄して解除を完了させる。
			u.logger.Warn("googlesync: build client for calendar delete failed", slog.String("error", cerr.Error()))
		} else if derr := client.DeleteCalendar(ctx, conn.CalendarID); derr != nil {
			u.logger.Warn("googlesync: delete calendar failed", slog.String("error", derr.Error()))
		}
	}

	if err := u.conns.Clear(ctx); err != nil {
		return err
	}
	return u.events.ClearAllGoogleMappings(ctx)
}

// resend は与えられたイベント群を専用カレンダーへ反映する（バックフィル・再同期で共用）。
// google_event_id 未保持なら作成、保持済みなら更新する。イベントが Google 側で手動削除
// （404）されていれば再作成する。失敗したイベントは sync_pending を維持する。
// トークン失効・専用カレンダー消失を検知した場合はそれ以上の送信を中止し、
// google.ErrTokenRevoked / google.ErrRemoteNotFound を返す（呼び出し側が未連携化する）。
func (u *UseCase) resend(ctx context.Context, client google.CalendarClient, calendarID string, events []*entity.CalendarEvent) error {
	tz := u.timezone(ctx)
	for _, ev := range events {
		data := google.EventData{
			Title:       ev.Title,
			Description: ev.Description,
			Location:    ev.Location,
			AllDay:      ev.AllDay,
			StartsAt:    ev.StartsAt,
			EndsAt:      ev.EndsAt,
			TimeZone:    tz,
		}
		if err := u.syncOne(ctx, client, calendarID, ev, data); err != nil {
			return err
		}
	}
	return nil
}

// syncOne は 1 イベントを反映する。イベント 404 は再作成し、失効/カレンダー 404 は
// 当該イベントを未同期にしたうえでそのエラーを返す（バッチ中止）。それ以外の失敗は
// 未同期にして nil を返し、次のイベントへ進む。
func (u *UseCase) syncOne(ctx context.Context, client google.CalendarClient, calendarID string, ev *entity.CalendarEvent, data google.EventData) error {
	if ev.GoogleEventID == "" {
		gid, err := client.CreateEvent(ctx, calendarID, data)
		if err != nil {
			return u.classifyResend(ctx, ev.ID, "", err, "create")
		}
		u.setSynced(ctx, ev.ID, gid)
		return nil
	}

	err := client.UpdateEvent(ctx, calendarID, ev.GoogleEventID, data)
	if err == nil {
		u.setSynced(ctx, ev.ID, ev.GoogleEventID)
		return nil
	}
	// イベントが Google 側で手動削除された → マッピングを捨てて再作成する。
	if errors.Is(err, google.ErrRemoteNotFound) {
		gid, cerr := client.CreateEvent(ctx, calendarID, data)
		if cerr != nil {
			return u.classifyResend(ctx, ev.ID, "", cerr, "recreate")
		}
		u.setSynced(ctx, ev.ID, gid)
		return nil
	}
	return u.classifyResend(ctx, ev.ID, ev.GoogleEventID, err, "update")
}

// classifyResend は失敗イベントを未同期にし、失効/カレンダー消失なら該当エラーを返して
// バッチを中止させる。一時的な失敗は nil を返して継続する。
func (u *UseCase) classifyResend(ctx context.Context, id int, googleEventID string, err error, op string) error {
	u.markPending(ctx, id, googleEventID)
	switch {
	case errors.Is(err, google.ErrTokenRevoked):
		u.logger.Warn("googlesync: token revoked", slog.String("op", op), slog.Int("id", id))
		return google.ErrTokenRevoked
	case errors.Is(err, google.ErrRemoteNotFound):
		u.logger.Warn("googlesync: calendar not found", slog.String("op", op), slog.Int("id", id))
		return google.ErrRemoteNotFound
	default:
		u.logger.Warn("googlesync: resend failed", slog.String("op", op), slog.Int("id", id), slog.String("error", err.Error()))
		return nil
	}
}

// handleResendFailure は resend が返した失効/カレンダー消失を連携状態へ反映し、
// 再連携が必要なら ErrReconnectRequired を返す。
func (u *UseCase) handleResendFailure(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, google.ErrTokenRevoked):
		if merr := u.conns.MarkDisconnected(ctx, false); merr != nil {
			u.logger.Warn("googlesync: mark disconnected failed", slog.String("error", merr.Error()))
		}
		return ErrReconnectRequired
	case errors.Is(err, google.ErrRemoteNotFound):
		if merr := u.conns.MarkDisconnected(ctx, true); merr != nil {
			u.logger.Warn("googlesync: mark disconnected failed", slog.String("error", merr.Error()))
		}
		if cerr := u.events.ClearAllGoogleMappings(ctx); cerr != nil {
			u.logger.Warn("googlesync: clear mappings failed", slog.String("error", cerr.Error()))
		}
		return ErrReconnectRequired
	default:
		return nil
	}
}

func (u *UseCase) setSynced(ctx context.Context, id int, googleEventID string) {
	if err := u.events.SetGoogleMapping(ctx, id, googleEventID, false); err != nil {
		u.logger.Warn("googlesync: persist mapping failed", slog.Int("id", id), slog.String("error", err.Error()))
	}
}

func (u *UseCase) markPending(ctx context.Context, id int, googleEventID string) {
	if err := u.events.SetGoogleMapping(ctx, id, googleEventID, true); err != nil {
		u.logger.Warn("googlesync: mark pending failed", slog.Int("id", id), slog.String("error", err.Error()))
	}
}

func (u *UseCase) timezone(ctx context.Context) string {
	if s, err := u.settings.Get(ctx); err == nil && s.Timezone != "" {
		return s.Timezone
	}
	return "UTC"
}
