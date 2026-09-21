package google

import (
	"context"
	"errors"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// CalendarName は専用カレンダーの名称（固定、design.md D7）。
const CalendarName = "CalendarRabbit"

// ErrTokenRevoked は refresh token 失効・権限取り消しを検知したときに返す。
// 呼び出し側はこれを判別して連携を未連携相当に落とす（design.md Risks）。
var ErrTokenRevoked = errors.New("google: token revoked or invalid_grant")

// ErrRemoteNotFound は Google 側のカレンダー/イベントが存在しない（404）ときに返す。
// 専用カレンダーやイベントがユーザーにより手動削除された場合に発生する。
// 呼び出し側はイベント 404 なら再作成、カレンダー 404 なら未連携化する等の回復を行う。
var ErrRemoteNotFound = errors.New("google: remote calendar or event not found")

// EventData は DB イベントを Google イベントへマッピングするための入力。
type EventData struct {
	Title       string
	Description string
	Location    string
	AllDay      bool
	StartsAt    time.Time
	EndsAt      time.Time
	// TimeZone は時刻付きイベントに付与する IANA タイムゾーン（例: Asia/Tokyo）。
	TimeZone string
}

// Token は OAuth トークンの最小表現（oauth2 型を呼び出し側に漏らさない）。
type Token struct {
	RefreshToken string
	AccessToken  string
	Expiry       time.Time
}

// CalendarClient は専用カレンダーとイベントの操作を抽象化する（フェイク化してテストする）。
type CalendarClient interface {
	CreateCalendar(ctx context.Context, summary string) (calendarID string, err error)
	DeleteCalendar(ctx context.Context, calendarID string) error
	CreateEvent(ctx context.Context, calendarID string, ev EventData) (eventID string, err error)
	UpdateEvent(ctx context.Context, calendarID, eventID string, ev EventData) error
	DeleteEvent(ctx context.Context, calendarID, eventID string) error
}

// ClientFactory は OAuth 補助と、連携状態からの認証済みクライアント生成を担う。
type ClientFactory interface {
	// AuthCodeURL は同意画面の認可 URL を返す。
	AuthCodeURL(state string) string
	// Exchange は認可コードをトークンへ交換する。
	Exchange(ctx context.Context, code string) (*Token, error)
	// Client は refresh token から認証済みの CalendarClient を生成する。
	Client(ctx context.Context, refreshToken string) (CalendarClient, error)
}

// service は calendar.Service を包んだ CalendarClient 実装。
type service struct {
	svc *calendar.Service
}

func (s *service) CreateCalendar(ctx context.Context, summary string) (string, error) {
	cal, err := s.svc.Calendars.Insert(&calendar.Calendar{Summary: summary}).Context(ctx).Do()
	if err != nil {
		return "", classifyErr(err)
	}
	return cal.Id, nil
}

func (s *service) DeleteCalendar(ctx context.Context, calendarID string) error {
	if err := s.svc.Calendars.Delete(calendarID).Context(ctx).Do(); err != nil {
		return classifyErr(err)
	}
	return nil
}

func (s *service) CreateEvent(ctx context.Context, calendarID string, ev EventData) (string, error) {
	created, err := s.svc.Events.Insert(calendarID, toGoogleEvent(ev)).Context(ctx).Do()
	if err != nil {
		return "", classifyErr(err)
	}
	return created.Id, nil
}

func (s *service) UpdateEvent(ctx context.Context, calendarID, eventID string, ev EventData) error {
	if _, err := s.svc.Events.Update(calendarID, eventID, toGoogleEvent(ev)).Context(ctx).Do(); err != nil {
		return classifyErr(err)
	}
	return nil
}

func (s *service) DeleteEvent(ctx context.Context, calendarID, eventID string) error {
	if err := s.svc.Events.Delete(calendarID, eventID).Context(ctx).Do(); err != nil {
		return classifyErr(err)
	}
	return nil
}

// toGoogleEvent は EventData を calendar.Event へマッピングする（design.md D5）。
// 終日イベントは date（日付のみ）、時刻付きイベントは dateTime + timeZone を用いる。
func toGoogleEvent(ev EventData) *calendar.Event {
	out := &calendar.Event{
		Summary:     ev.Title,
		Description: ev.Description,
		Location:    ev.Location,
	}
	if ev.AllDay {
		out.Start = &calendar.EventDateTime{Date: ev.StartsAt.Format("2006-01-02")}
		// Google の終日 end.date は排他的（その日を含まない）。CalendarRabbit は ends_at を
		// 最終日を含む（inclusive）値で保持しているため +1 日して最終日まで反映させる
		// （フロントの react-big-calendar と同じ扱い）。
		out.End = &calendar.EventDateTime{Date: ev.EndsAt.AddDate(0, 0, 1).Format("2006-01-02")}
	} else {
		out.Start = &calendar.EventDateTime{DateTime: ev.StartsAt.Format(time.RFC3339), TimeZone: ev.TimeZone}
		out.End = &calendar.EventDateTime{DateTime: ev.EndsAt.Format(time.RFC3339), TimeZone: ev.TimeZone}
	}
	return out
}

// classifyErr は認可失敗（401 / invalid_grant）を ErrTokenRevoked に正規化する。
func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	var gapiErr *googleapi.Error
	if errors.As(err, &gapiErr) {
		switch gapiErr.Code {
		case http.StatusUnauthorized, http.StatusForbidden:
			return errors.Join(ErrTokenRevoked, err)
		case http.StatusNotFound:
			return errors.Join(ErrRemoteNotFound, err)
		}
	}
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) {
		return errors.Join(ErrTokenRevoked, err)
	}
	return err
}

// ensure the real service satisfies the interface.
var _ CalendarClient = (*service)(nil)

// newService は HTTP クライアントから calendar.Service を包む CalendarClient を作る。
func newService(ctx context.Context, opts ...option.ClientOption) (CalendarClient, error) {
	svc, err := calendar.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &service{svc: svc}, nil
}
