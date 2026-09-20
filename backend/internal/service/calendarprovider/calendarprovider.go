// Package calendarprovider はカレンダー永続化の抽象境界を定義する。
// 本change では DB 実装のみを提供し、将来 Google Calendar 等の実装を
// 追加してもusecase層を変更不要にする（design.md D5）。
package calendarprovider

import (
	"context"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
)

// EventInput はイベントの作成・更新の入力。
type EventInput struct {
	Title       string
	StartsAt    time.Time
	EndsAt      time.Time
	AllDay      bool
	Location    string
	Description string
	SourceURL   string
}

// CalendarProvider はカレンダーイベントの永続化を抽象化する。
type CalendarProvider interface {
	Create(ctx context.Context, in EventInput) (*entity.CalendarEvent, error)
	Update(ctx context.Context, id int, in EventInput) (*entity.CalendarEvent, error)
	Delete(ctx context.Context, id int) error
	Get(ctx context.Context, id int) (*entity.CalendarEvent, error)
	List(ctx context.Context) ([]*entity.CalendarEvent, error)
	ListByPeriod(ctx context.Context, from, to time.Time) ([]*entity.CalendarEvent, error)
}

// DBProvider は DB（ent）を用いた CalendarProvider 実装。
//
// 各メソッドは persistence.CalendarEventRepository への薄い委譲であり、
// 計装（logging.Trace）は行わない。DB 呼び出しの前後ログは委譲先の
// persistence 層に集約しており（4.1）、ここで計装すると同一 DB 操作に対して
// 二重の started/finished ログが出てしまうためである（design D4）。
type DBProvider struct {
	repo *persistence.CalendarEventRepository
}

// NewDBProvider は DBProvider を生成する。
func NewDBProvider(repo *persistence.CalendarEventRepository) *DBProvider {
	return &DBProvider{repo: repo}
}

var _ CalendarProvider = (*DBProvider)(nil)

func (p *DBProvider) Create(ctx context.Context, in EventInput) (*entity.CalendarEvent, error) {
	return p.repo.Create(ctx, toRepoInput(in))
}

func (p *DBProvider) Update(ctx context.Context, id int, in EventInput) (*entity.CalendarEvent, error) {
	return p.repo.Update(ctx, id, toRepoInput(in))
}

func (p *DBProvider) Delete(ctx context.Context, id int) error {
	return p.repo.Delete(ctx, id)
}

func (p *DBProvider) Get(ctx context.Context, id int) (*entity.CalendarEvent, error) {
	return p.repo.Get(ctx, id)
}

func (p *DBProvider) List(ctx context.Context) ([]*entity.CalendarEvent, error) {
	return p.repo.List(ctx)
}

func (p *DBProvider) ListByPeriod(ctx context.Context, from, to time.Time) ([]*entity.CalendarEvent, error) {
	return p.repo.ListByPeriod(ctx, from, to)
}

func toRepoInput(in EventInput) persistence.CalendarEventInput {
	return persistence.CalendarEventInput{
		Title:       in.Title,
		StartsAt:    in.StartsAt,
		EndsAt:      in.EndsAt,
		AllDay:      in.AllDay,
		Location:    in.Location,
		Description: in.Description,
		SourceURL:   in.SourceURL,
	}
}
