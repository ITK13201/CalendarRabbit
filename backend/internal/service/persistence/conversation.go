package persistence

import (
	"context"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/ent/conversation"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
)

// ConversationRepository は単一連続スレッド（Conversation）の永続化を担う。
type ConversationRepository struct {
	client *ent.Client
}

func NewConversationRepository(client *ent.Client) *ConversationRepository {
	return &ConversationRepository{client: client}
}

// GetOrCreate は既存の会話（最古の1件）を返す。無ければ新規作成する。
// 単一連続スレッド前提のため、常に同一スレッドを返す。
func (r *ConversationRepository) GetOrCreate(ctx context.Context) (*entity.Conversation, error) {
	row, err := r.client.Conversation.Query().
		Order(ent.Asc(conversation.FieldID)).
		First(ctx)
	if err == nil {
		return mapConversation(row), nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	created, err := r.client.Conversation.Create().Save(ctx)
	if err != nil {
		return nil, err
	}
	return mapConversation(created), nil
}

// Get は指定IDの会話を取得する。
func (r *ConversationRepository) Get(ctx context.Context, id int) (*entity.Conversation, error) {
	row, err := r.client.Conversation.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, derr.ErrNotFound
		}
		return nil, err
	}
	return mapConversation(row), nil
}

func mapConversation(row *ent.Conversation) *entity.Conversation {
	return &entity.Conversation{
		ID:        row.ID,
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}
}
