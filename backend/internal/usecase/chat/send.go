package chat

import (
	"context"
	"log/slog"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	chatservice "github.com/ITK13201/CalendarRabbit/backend/internal/service/chat"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
)

// maxHistoryTurns は Claude へ渡す会話履歴の上限ターン数。
// 会話が伸びてもコンテキスト（トークン）が際限なく増えないよう、直近この件数に制限する。
const maxHistoryTurns = 10

// SendMessage はユーザーメッセージを受け付け、会話に永続化し、
// Claude によるイベント抽出結果に応じて予定案を生成する。
func (u *UseCase) SendMessage(ctx context.Context, content string) (res *SendResult, err error) {
	defer logging.Trace(ctx, u.logger, "chat.UseCase.SendMessage",
		logging.Args{"content": content}, &res, &err)()

	convRepo := persistence.NewConversationRepository(u.client, u.logger)
	msgRepo := persistence.NewMessageRepository(u.client, u.logger)
	propRepo := persistence.NewEventProposalRepository(u.client, u.logger)

	conv, err := convRepo.GetOrCreate(ctx)
	if err != nil {
		return nil, err
	}

	// 抽出には現在のメッセージを含めない過去履歴を渡す。
	history, err := u.buildHistory(ctx, msgRepo, conv.ID)
	if err != nil {
		return nil, err
	}

	userMsg, err := msgRepo.Create(ctx, conv.ID, entity.RoleUser, content)
	if err != nil {
		return nil, err
	}

	extraction, err := u.resolveExtractor(ctx).Extract(ctx, history, content)
	if err != nil {
		return nil, err
	}

	assistantMsg, err := msgRepo.Create(ctx, conv.ID, entity.RoleAssistant, extraction.Message)
	if err != nil {
		return nil, err
	}

	result := &SendResult{UserMessage: userMsg, AssistantMessage: assistantMsg}

	switch extraction.Status {
	case chatservice.StatusEvent:
		ev := extraction.Event
		proposal, perr := propRepo.Create(ctx, persistence.EventProposalInput{
			ConversationID: conv.ID,
			MessageID:      &assistantMsg.ID,
			Title:          ev.Title,
			StartsAt:       ev.StartsAt,
			EndsAt:         ev.EndsAt,
			AllDay:         ev.AllDay,
			Location:       ev.Location,
			Description:    ev.Description,
			SourceURL:      ev.SourceURL,
		})
		if perr != nil {
			return nil, perr
		}
		result.Proposal = proposal
		logging.LogContext(ctx, u.logger, slog.LevelInfo, "chat.proposal_created", slog.Int("proposal_id", proposal.ID))
	case chatservice.StatusMultiple:
		for _, c := range extraction.Candidates {
			result.Candidates = append(result.Candidates, Candidate(c))
		}
		logging.LogContext(ctx, u.logger, slog.LevelInfo, "chat.multiple_candidates", slog.Int("count", len(result.Candidates)))
	case chatservice.StatusNotFound:
		logging.LogContext(ctx, u.logger, slog.LevelInfo, "chat.not_found")
	case chatservice.StatusOffTopic:
		logging.LogContext(ctx, u.logger, slog.LevelInfo, "chat.off_topic")
	}

	return result, nil
}

// buildHistory は会話の既存メッセージを Turn 列に変換する。
func (u *UseCase) buildHistory(ctx context.Context, msgRepo *persistence.MessageRepository, conversationID int) ([]chatservice.Turn, error) {
	msgs, err := msgRepo.ListByConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	// 直近 maxHistoryTurns ターンのみを送信対象とする（末尾を残す）。
	if len(msgs) > maxHistoryTurns {
		msgs = msgs[len(msgs)-maxHistoryTurns:]
	}
	turns := make([]chatservice.Turn, 0, len(msgs))
	for _, m := range msgs {
		turns = append(turns, chatservice.Turn{Role: m.Role, Content: m.Content})
	}
	return turns, nil
}
