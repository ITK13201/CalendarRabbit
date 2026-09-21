package calendarprovider

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/google"
)

// ConnectionReader は連携状態の取得と、失効/カレンダー消失時の未連携化を行う
// （persistence.GoogleConnectionRepository が満たす）。
type ConnectionReader interface {
	Get(ctx context.Context) (*entity.GoogleConnection, error)
	MarkDisconnected(ctx context.Context, clearCalendar bool) error
}

// SettingReader はタイムゾーン取得のための設定リーダ（persistence.AppSettingRepository が満たす）。
type SettingReader interface {
	Get(ctx context.Context) (*entity.AppSetting, error)
}

// MappingWriter はイベントの Google 同期状態を更新する（persistence.CalendarEventRepository が満たす）。
type MappingWriter interface {
	SetGoogleMapping(ctx context.Context, id int, googleEventID string, syncPending bool) error
	ClearAllGoogleMappings(ctx context.Context) error
}

// MirroringProvider は内側の CalendarProvider（DB）に委譲したうえで、
// Google 連携が有効な場合に専用カレンダーへ即時ミラー同期するデコレータ（design.md D1, D4）。
//
// DB を正とするため、Google 同期が失敗しても DB 操作は成功として返し、
// 当該イベントに sync_pending=true を立てる。
type MirroringProvider struct {
	inner    CalendarProvider
	conns    ConnectionReader
	settings SettingReader
	events   MappingWriter
	factory  google.ClientFactory
	logger   *slog.Logger
}

// NewMirroringProvider は MirroringProvider を生成する。
func NewMirroringProvider(
	inner CalendarProvider,
	conns ConnectionReader,
	settings SettingReader,
	events MappingWriter,
	factory google.ClientFactory,
	logger *slog.Logger,
) *MirroringProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &MirroringProvider{
		inner:    inner,
		conns:    conns,
		settings: settings,
		events:   events,
		factory:  factory,
		logger:   logger,
	}
}

var _ CalendarProvider = (*MirroringProvider)(nil)

func (p *MirroringProvider) Create(ctx context.Context, in EventInput) (*entity.CalendarEvent, error) {
	ev, err := p.inner.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	p.mirrorUpsert(ctx, ev)
	return ev, nil
}

func (p *MirroringProvider) Update(ctx context.Context, id int, in EventInput) (*entity.CalendarEvent, error) {
	ev, err := p.inner.Update(ctx, id, in)
	if err != nil {
		return nil, err
	}
	p.mirrorUpsert(ctx, ev)
	return ev, nil
}

func (p *MirroringProvider) Delete(ctx context.Context, id int) error {
	// 削除前に google_event_id を取得しておく（削除後は参照できないため）。
	existing, getErr := p.inner.Get(ctx, id)
	if err := p.inner.Delete(ctx, id); err != nil {
		return err
	}
	if getErr != nil || existing == nil || existing.GoogleEventID == "" {
		return nil
	}
	conn := p.activeConnection(ctx)
	if conn == nil {
		return nil
	}
	client, err := p.factory.Client(ctx, conn.RefreshToken)
	if err != nil {
		p.logger.Warn("google mirror: build client for delete failed", slog.String("error", err.Error()))
		return nil
	}
	if err := client.DeleteEvent(ctx, conn.CalendarID, existing.GoogleEventID); err != nil {
		// 行は削除済みで sync_pending を立てる先が無いため best-effort。
		// ただし失効・カレンダー消失は連携状態へ反映する。
		switch {
		case errors.Is(err, google.ErrRemoteNotFound):
			// 既に Google 側に無い＝削除の目的は達成済み。
		case errors.Is(err, google.ErrTokenRevoked):
			p.disconnect(ctx, false, "delete")
		default:
			p.logger.Warn("google mirror: delete event failed", slog.Int("id", id), slog.String("error", err.Error()))
		}
	}
	return nil
}

func (p *MirroringProvider) Get(ctx context.Context, id int) (*entity.CalendarEvent, error) {
	return p.inner.Get(ctx, id)
}

func (p *MirroringProvider) List(ctx context.Context) ([]*entity.CalendarEvent, error) {
	return p.inner.List(ctx)
}

func (p *MirroringProvider) ListByPeriod(ctx context.Context, from, to time.Time) ([]*entity.CalendarEvent, error) {
	return p.inner.ListByPeriod(ctx, from, to)
}

// mirrorUpsert は Create/Update 後のミラー同期を行う。連携が無効なら何もしない。
// 失敗時は DB 操作を成功のまま保ち、当該イベントに sync_pending=true を立てる（design.md D4）。
func (p *MirroringProvider) mirrorUpsert(ctx context.Context, ev *entity.CalendarEvent) {
	conn := p.activeConnection(ctx)
	if conn == nil {
		return
	}
	client, err := p.factory.Client(ctx, conn.RefreshToken)
	if err != nil {
		p.markPending(ctx, ev)
		p.logger.Warn("google mirror: build client failed", slog.String("error", err.Error()))
		return
	}

	data := p.toEventData(ctx, ev)
	if ev.GoogleEventID == "" {
		p.create(ctx, client, conn, ev, data)
		return
	}

	uerr := client.UpdateEvent(ctx, conn.CalendarID, ev.GoogleEventID, data)
	if uerr == nil {
		p.setMapping(ctx, ev, ev.GoogleEventID)
		return
	}
	// イベントが Google 側で手動削除された場合は 404 になる → マッピングを捨てて再作成する。
	if errors.Is(uerr, google.ErrRemoteNotFound) {
		p.logger.Warn("google mirror: event not found, recreating", slog.Int("id", ev.ID))
		ev.GoogleEventID = ""
		p.create(ctx, client, conn, ev, data)
		return
	}
	p.handleSyncErr(ctx, ev, uerr, "update")
}

// create は新規イベントを Google へ作成し、失敗時は失効/カレンダー消失を判別して連携状態へ反映する。
func (p *MirroringProvider) create(ctx context.Context, client google.CalendarClient, conn *entity.GoogleConnection, ev *entity.CalendarEvent, data google.EventData) {
	gid, err := client.CreateEvent(ctx, conn.CalendarID, data)
	if err != nil {
		p.handleSyncErr(ctx, ev, err, "create")
		return
	}
	p.setMapping(ctx, ev, gid)
}

// handleSyncErr はミラー失敗を分類し、当該イベントを未同期にしたうえで
// トークン失効・専用カレンダー消失を連携状態へ反映する（A/B 回復）。
func (p *MirroringProvider) handleSyncErr(ctx context.Context, ev *entity.CalendarEvent, err error, op string) {
	p.markPending(ctx, ev)
	switch {
	case errors.Is(err, google.ErrTokenRevoked):
		p.logger.Warn("google mirror: token revoked", slog.String("op", op), slog.Int("id", ev.ID))
		p.disconnect(ctx, false, op)
	case errors.Is(err, google.ErrRemoteNotFound):
		// 作成で 404 ＝ 専用カレンダーが手動削除された。未連携化し calendar_id とマッピングを破棄する。
		p.logger.Warn("google mirror: calendar not found", slog.String("op", op), slog.Int("id", ev.ID))
		p.disconnect(ctx, true, op)
		if cerr := p.events.ClearAllGoogleMappings(ctx); cerr != nil {
			p.logger.Warn("google mirror: clear mappings failed", slog.String("error", cerr.Error()))
		}
	default:
		p.logger.Warn("google mirror: sync failed", slog.String("op", op), slog.Int("id", ev.ID), slog.String("error", err.Error()))
	}
}

// disconnect は連携を未連携相当に落とす（status で再連携を促すため）。
func (p *MirroringProvider) disconnect(ctx context.Context, clearCalendar bool, op string) {
	if err := p.conns.MarkDisconnected(ctx, clearCalendar); err != nil {
		p.logger.Warn("google mirror: mark disconnected failed", slog.String("op", op), slog.String("error", err.Error()))
	}
}

func (p *MirroringProvider) setMapping(ctx context.Context, ev *entity.CalendarEvent, googleEventID string) {
	if err := p.events.SetGoogleMapping(ctx, ev.ID, googleEventID, false); err != nil {
		p.logger.Warn("google mirror: persist mapping failed", slog.Int("id", ev.ID), slog.String("error", err.Error()))
		return
	}
	ev.GoogleEventID = googleEventID
	ev.SyncPending = false
}

func (p *MirroringProvider) markPending(ctx context.Context, ev *entity.CalendarEvent) {
	if err := p.events.SetGoogleMapping(ctx, ev.ID, ev.GoogleEventID, true); err != nil {
		p.logger.Warn("google mirror: mark pending failed", slog.Int("id", ev.ID), slog.String("error", err.Error()))
		return
	}
	ev.SyncPending = true
}

// activeConnection は連携が有効かつ refresh token を持つ場合のみ接続を返す。それ以外は nil。
func (p *MirroringProvider) activeConnection(ctx context.Context) *entity.GoogleConnection {
	conn, err := p.conns.Get(ctx)
	if err != nil {
		if !errors.Is(err, derr.ErrNotFound) {
			p.logger.Warn("google mirror: load connection failed", slog.String("error", err.Error()))
		}
		return nil
	}
	if conn == nil || !conn.Connected || conn.RefreshToken == "" || conn.CalendarID == "" {
		return nil
	}
	return conn
}

// toEventData は DB イベントを Google 用の入力へ変換する。タイムゾーンは設定から取得する（design.md D5）。
func (p *MirroringProvider) toEventData(ctx context.Context, ev *entity.CalendarEvent) google.EventData {
	tz := "UTC"
	if s, err := p.settings.Get(ctx); err == nil && s.Timezone != "" {
		tz = s.Timezone
	}
	return google.EventData{
		Title:       ev.Title,
		Description: ev.Description,
		Location:    ev.Location,
		AllDay:      ev.AllDay,
		StartsAt:    ev.StartsAt,
		EndsAt:      ev.EndsAt,
		TimeZone:    tz,
	}
}
