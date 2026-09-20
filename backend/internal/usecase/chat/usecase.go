// Package chat はチャット駆動の予定登録フロー（メッセージ送信・予定案生成・承認・却下）の
// ユースケースを提供する。
package chat

import (
	"context"
	"log/slog"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	chatservice "github.com/ITK13201/CalendarRabbit/backend/internal/service/chat"
)

// SettingsReader は現在のアプリ設定（選択中の LLM プロバイダ）を読み取る。
type SettingsReader interface {
	Get(ctx context.Context) (*entity.AppSetting, error)
}

// UseCase はチャット予定登録フローのユースケース。
type UseCase struct {
	client *ent.Client
	// extractors はプロバイダ名 -> Extractor のマップ（設定に応じて実行時に選択）。
	extractors      map[string]chatservice.Extractor
	settings        SettingsReader
	defaultProvider string
	logger          *slog.Logger
}

// New は UseCase を生成する。extractors は利用可能な各プロバイダの Extractor、
// settings は選択中プロバイダの読み取り元、defaultProvider は設定不明時のフォールバック。
func New(client *ent.Client, extractors map[string]chatservice.Extractor, settings SettingsReader, defaultProvider string, logger *slog.Logger) *UseCase {
	return &UseCase{
		client:          client,
		extractors:      extractors,
		settings:        settings,
		defaultProvider: defaultProvider,
		logger:          logger,
	}
}

// resolveExtractor は設定に応じた Extractor を返す。設定取得や該当プロバイダが無い場合は
// defaultProvider のものへフォールバックする。
func (u *UseCase) resolveExtractor(ctx context.Context) chatservice.Extractor {
	provider := u.defaultProvider
	if u.settings != nil {
		if s, err := u.settings.Get(ctx); err == nil && s.LLMProvider != "" {
			provider = s.LLMProvider
		}
	}
	if ex, ok := u.extractors[provider]; ok {
		return ex
	}
	return u.extractors[u.defaultProvider]
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
