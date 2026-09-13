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
