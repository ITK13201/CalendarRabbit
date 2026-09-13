// Package chat は Claude API + web_search によるイベント抽出サービスを提供する。
package chat

import (
	"context"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
)

// ExtractionStatus は抽出結果の種別。
type ExtractionStatus string

const (
	// StatusEvent はイベントを1件特定できた。
	StatusEvent ExtractionStatus = "event"
	// StatusMultiple は複数候補が存在する。
	StatusMultiple ExtractionStatus = "multiple"
	// StatusNotFound はイベントを特定できなかった。
	StatusNotFound ExtractionStatus = "not_found"
	// StatusOffTopic は予定登録と無関係なメッセージ。
	StatusOffTopic ExtractionStatus = "off_topic"
)

// ExtractedEvent は抽出されたイベント情報。
type ExtractedEvent struct {
	Title       string
	StartsAt    time.Time
	EndsAt      time.Time
	AllDay      bool
	Location    string
	Description string
	SourceURL   string
}

// ExtractionResult は Claude によるイベント抽出結果。
type ExtractionResult struct {
	Status     ExtractionStatus
	Message    string // ユーザーに提示するアシスタント応答
	Event      *ExtractedEvent
	Candidates []ExtractedEvent
}

// Turn は会話履歴の1ターン。
type Turn struct {
	Role    entity.Role
	Content string
}

// Extractor はユーザーメッセージからイベントを抽出する。
type Extractor interface {
	Extract(ctx context.Context, history []Turn, userMessage string) (*ExtractionResult, error)
}
