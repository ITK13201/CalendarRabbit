// Package entity はアプリのドメインエンティティを定義する。
// ent の生成型に依存せず、usecase/handler 層はこの型を用いる。
package entity

import "time"

// Role はチャットメッセージの発話者。
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ProposalStatus は予定案の承認状態機械。
type ProposalStatus string

const (
	ProposalStatusPending  ProposalStatus = "pending"
	ProposalStatusApproved ProposalStatus = "approved"
	ProposalStatusRejected ProposalStatus = "rejected"
)

// Conversation は単一連続チャットスレッド。
type Conversation struct {
	ID        int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message はチャットの1メッセージ。
type Message struct {
	ID             int
	ConversationID int
	Role           Role
	Content        string
	CreatedAt      time.Time
}

// EventProposal はチャットから生成された予定案。
type EventProposal struct {
	ID              int
	ConversationID  int
	MessageID       *int
	Title           string
	StartsAt        time.Time
	EndsAt          time.Time
	AllDay          bool
	Location        string
	Description     string
	SourceURL       string
	Status          ProposalStatus
	CalendarEventID *int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CalendarEvent は登録済みのカレンダーイベント（日時はUTC）。
type CalendarEvent struct {
	ID          int
	Title       string
	StartsAt    time.Time
	EndsAt      time.Time
	AllDay      bool
	Location    string
	Description string
	SourceURL   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AppSetting はアプリ設定（単一レコード）。
type AppSetting struct {
	ID          int
	Timezone    string
	LLMProvider string
	UpdatedAt   time.Time
}
