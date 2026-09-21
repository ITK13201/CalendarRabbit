package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// GoogleConnection は Google Calendar 連携の状態を保持する単一レコード（id=1固定運用）。
// 単一ユーザー・単一 Google アカウント前提のため、AppSetting と同様に常に1レコードで表現する。
//
// refresh_token はアプリ層で暗号化した文字列を保存する（平文保存しない、design.md D3b）。
type GoogleConnection struct {
	ent.Schema
}

func (GoogleConnection) Fields() []ent.Field {
	return []ent.Field{
		// アプリ層で暗号化した refresh token（未連携時は空）。
		field.Text("refresh_token").
			Default(""),
		// キャッシュした access token（未連携時は空）。
		field.Text("access_token").
			Default(""),
		// access token の失効時刻（未設定可）。
		field.Time("token_expiry").
			Optional(),
		// 専用カレンダーの calendarId（未作成時は空）。
		field.String("calendar_id").
			Default(""),
		// 連携が有効かどうか。
		field.Bool("connected").
			Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}
