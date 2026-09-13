// Package chat はチャット駆動の予定登録フロー（メッセージ送信・予定案生成・承認・却下）の
// ユースケースを提供する。
package chat

import (
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	chatservice "github.com/ITK13201/CalendarRabbit/backend/internal/service/chat"
)

// UseCase はチャット予定登録フローのユースケース。
type UseCase struct {
	client    *ent.Client
	extractor chatservice.Extractor
	logger    *slog.Logger
}

// New は UseCase を生成する。
func New(client *ent.Client, extractor chatservice.Extractor, logger *slog.Logger) *UseCase {
	return &UseCase{client: client, extractor: extractor, logger: logger}
}

// SendResult はメッセージ送信の結果。
type SendResult struct {
	UserMessage      *entity.Message
	AssistantMessage *entity.Message
	// Proposal はイベントを1件特定できた場合の pending 予定案（それ以外は nil）。
	Proposal *entity.EventProposal
	// Candidates は複数候補時の候補一覧（永続化しない。ユーザーの選択を促す用途）。
	Candidates []Candidate
}

// Candidate は複数候補時の1候補。
type Candidate struct {
	Title       string
	StartsAt    time.Time
	EndsAt      time.Time
	AllDay      bool
	Location    string
	Description string
	SourceURL   string
}

// ConversationView は会話履歴の取得結果。
type ConversationView struct {
	Conversation *entity.Conversation
	Messages     []*entity.Message
	Proposals    []*entity.EventProposal
}

// ProposalEdit は承認時にユーザーが編集した内容（任意）。
type ProposalEdit struct {
	Title       string
	StartsAt    time.Time
	EndsAt      time.Time
	AllDay      bool
	Location    string
	Description string
	SourceURL   string
}
