package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// CalendarEvent は登録済みのカレンダーイベント。日時はUTCで保存する。
type CalendarEvent struct {
	ent.Schema
}

func (CalendarEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").
			NotEmpty(),
		field.Time("starts_at"),
		field.Time("ends_at"),
		field.Bool("all_day").
			Default(false),
		field.String("location").
			Default(""),
		field.Text("description").
			Default(""),
		field.String("source_url").
			Default(""),
		// google_event_id は専用カレンダー上の対応イベントID（未同期・未連携時は空）。
		field.String("google_event_id").
			Default(""),
		// sync_pending は Google への同期が未完了（失敗または未実行）であることを示す。
		field.Bool("sync_pending").
			Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (CalendarEvent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("proposal", EventProposal.Type).
			Ref("calendar_event"),
	}
}
