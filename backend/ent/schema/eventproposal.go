package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// EventProposal はチャットから生成された予定案。承認フローの状態機械を持つ。
type EventProposal struct {
	ent.Schema
}

func (EventProposal) Fields() []ent.Field {
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
		field.Enum("status").
			Values("pending", "approved", "rejected").
			Default("pending"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (EventProposal) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("conversation", Conversation.Type).
			Ref("proposals").
			Unique().
			Required(),
		edge.From("message", Message.Type).
			Ref("proposals").
			Unique(),
		// 承認後に作成された CalendarEvent への参照（nullable）。
		edge.To("calendar_event", CalendarEvent.Type).
			Unique(),
	}
}
