package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Conversation は単一連続チャットスレッドを表す（実質1レコード）。
type Conversation struct {
	ent.Schema
}

func (Conversation) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (Conversation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("messages", Message.Type),
		edge.To("proposals", EventProposal.Type),
	}
}
