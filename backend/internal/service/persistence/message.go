package persistence

import (
	"context"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/ent/conversation"
	"github.com/ITK13201/CalendarRabbit/backend/ent/message"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
)

// MessageRepository はチャットメッセージの永続化を担う。
type MessageRepository struct {
	client *ent.Client
}

func NewMessageRepository(client *ent.Client) *MessageRepository {
	return &MessageRepository{client: client}
}

// Create は会話にメッセージを追記する。
func (r *MessageRepository) Create(ctx context.Context, conversationID int, role entity.Role, content string) (*entity.Message, error) {
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
func (r *MessageRepository) ListByConversation(ctx context.Context, conversationID int) ([]*entity.Message, error) {
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

func mapMessage(row *ent.Message, conversationID int) *entity.Message {
	return &entity.Message{
		ID:             row.ID,
		ConversationID: conversationID,
		Role:           entity.Role(row.Role),
		Content:        row.Content,
		CreatedAt:      row.CreatedAt.UTC(),
	}
}
