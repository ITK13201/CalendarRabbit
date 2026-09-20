package chat

import (
	"context"
	"log/slog"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
)

// ClearConversation は単一連続スレッドのメッセージと予定案をすべて削除する。
// 外部参照の依存順（event_proposal → message）でトランザクション内で削除する。
// 会話レコード自体は残し、以降のメッセージ送信は空の状態から再開する。
func (u *UseCase) ClearConversation(ctx context.Context) (err error) {
	defer logging.Trace(ctx, u.logger, "chat.UseCase.ClearConversation", nil, nil, &err)()

	convRepo := persistence.NewConversationRepository(u.client, u.logger)
	conv, err := convRepo.GetOrCreate(ctx)
	if err != nil {
		return err
	}

	if err := persistence.WithTx(ctx, u.client, func(txClient *ent.Client) error {
		propRepo := persistence.NewEventProposalRepository(txClient, u.logger)
		msgRepo := persistence.NewMessageRepository(txClient, u.logger)
		if derr := propRepo.DeleteByConversation(ctx, conv.ID); derr != nil {
			return derr
		}
		return msgRepo.DeleteByConversation(ctx, conv.ID)
	}); err != nil {
		return err
	}

	logging.LogContext(ctx, u.logger, slog.LevelInfo, "chat.conversation_cleared", slog.Int("conversation_id", conv.ID))
	return nil
}
