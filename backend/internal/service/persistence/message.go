package persistence

import (
	"context"
	"log/slog"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/ent/conversation"
	"github.com/ITK13201/CalendarRabbit/backend/ent/message"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
)

// MessageRepository はチャットメッセージの永続化を担う。
type MessageRepository struct {
	client *ent.Client
	logger *slog.Logger
}

func NewMessageRepository(client *ent.Client, logger *slog.Logger) *MessageRepository {
	return &MessageRepository{client: client, logger: logger}
}

// Create は会話にメッセージを追記する。
func (r *MessageRepository) Create(ctx context.Context, conversationID int, role entity.Role, content string) (res *entity.Message, err error) {
	defer logging.Trace(ctx, r.logger, "persistence.MessageRepository.Create",
		logging.Args{"conversationID": conversationID, "role": role, "content": content}, &res, &err)()

	row, err := r.client.Message.Create().
		SetConversationID(conversationID).
		SetRole(message.Role(role)).
		SetContent(content).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return mapMessage(row, conversationID), nil
}

// ListByConversation は会話の全メッセージを時系列（作成日時→ID）順で返す。
func (r *MessageRepository) ListByConversation(ctx context.Context, conversationID int) (res []*entity.Message, err error) {
	defer logging.Trace(ctx, r.logger, "persistence.MessageRepository.ListByConversation",
		logging.Args{"conversationID": conversationID}, &res, &err)()

	rows, err := r.client.Message.Query().
		Where(message.HasConversationWith(conversation.IDEQ(conversationID))).
		Order(ent.Asc(message.FieldCreatedAt), ent.Asc(message.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*entity.Message, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapMessage(row, conversationID))
	}
	return out, nil
}

// DeleteByConversation は会話に属する全メッセージを削除する。
func (r *MessageRepository) DeleteByConversation(ctx context.Context, conversationID int) (err error) {
	defer logging.Trace(ctx, r.logger, "persistence.MessageRepository.DeleteByConversation",
		logging.Args{"conversationID": conversationID}, nil, &err)()

	_, err = r.client.Message.Delete().
		Where(message.HasConversationWith(conversation.IDEQ(conversationID))).
		Exec(ctx)
	return err
}

func mapMessage(row *ent.Message, conversationID int) *entity.Message {
	return &entity.Message{
		ID:             row.ID,
		ConversationID: conversationID,
		Role:           entity.Role(row.Role),
		Content:        row.Content,
		CreatedAt:      row.CreatedAt.UTC(),
	}
}
