package chat

import (
	"context"

	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
)

// GetConversation は単一連続スレッドの全メッセージと予定案を時系列で返す。
func (u *UseCase) GetConversation(ctx context.Context) (res *ConversationView, err error) {
	defer logging.Trace(ctx, u.logger, "chat.UseCase.GetConversation", nil, &res, &err)()

	convRepo := persistence.NewConversationRepository(u.client, u.logger)
	msgRepo := persistence.NewMessageRepository(u.client, u.logger)
	propRepo := persistence.NewEventProposalRepository(u.client, u.logger)

	conv, err := convRepo.GetOrCreate(ctx)
	if err != nil {
		return nil, err
	}
	messages, err := msgRepo.ListByConversation(ctx, conv.ID)
	if err != nil {
		return nil, err
	}
	proposals, err := propRepo.ListByConversation(ctx, conv.ID)
	if err != nil {
		return nil, err
	}
	return &ConversationView{Conversation: conv, Messages: messages, Proposals: proposals}, nil
}
