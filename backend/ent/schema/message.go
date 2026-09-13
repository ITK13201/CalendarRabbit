package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Message はチャット上の1メッセージ（ユーザー or アシスタント）。
type Message struct {
	ent.Schema
}

func (Message) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("role").
			Values("user", "assistant"),
		field.Text("content"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Message) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("conversation", Conversation.Type).
			Ref("messages").
			Unique().
			Required(),
		edge.To("proposals", EventProposal.Type),
	}
}
