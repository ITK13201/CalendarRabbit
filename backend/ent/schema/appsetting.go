package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// AppSetting はアプリ設定の単一レコード（id=1固定運用）。
type AppSetting struct {
	ent.Schema
}

func (AppSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("timezone").
			Default("Asia/Tokyo").
			NotEmpty(),
		// llm_provider は使用する LLM プロバイダ（"deepseek" | "claude"）。既定は deepseek。
		field.Enum("llm_provider").
			Values("deepseek", "claude").
			Default("deepseek"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}
