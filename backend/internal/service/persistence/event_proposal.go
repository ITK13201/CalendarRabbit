package persistence

import (
	"context"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/ent/conversation"
	"github.com/ITK13201/CalendarRabbit/backend/ent/eventproposal"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
)

// EventProposalRepository は予定案（EventProposal）の永続化を担う。
type EventProposalRepository struct {
	client *ent.Client
}

func NewEventProposalRepository(client *ent.Client) *EventProposalRepository {
	return &EventProposalRepository{client: client}
}

// EventProposalInput は予定案作成の入力。
type EventProposalInput struct {
	ConversationID int
	MessageID      *int
	Title          string
	StartsAt       time.Time
	EndsAt         time.Time
	AllDay         bool
	Location       string
	Description    string
	SourceURL      string
}

// Create は予定案を pending 状態で作成する。
func (r *EventProposalRepository) Create(ctx context.Context, in EventProposalInput) (*entity.EventProposal, error) {
	builder := r.client.EventProposal.Create().
		SetConversationID(in.ConversationID).
		SetTitle(in.Title).
		SetStartsAt(in.StartsAt.UTC()).
		SetEndsAt(in.EndsAt.UTC()).
		SetAllDay(in.AllDay).
		SetLocation(in.Location).
		SetDescription(in.Description).
		SetSourceURL(in.SourceURL).
		SetStatus(eventproposal.StatusPending)
	if in.MessageID != nil {
		builder = builder.SetMessageID(*in.MessageID)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, row.ID)
}

// Get は予定案を取得する（関連IDを含む）。
func (r *EventProposalRepository) Get(ctx context.Context, id int) (*entity.EventProposal, error) {
	row, err := r.client.EventProposal.Query().
		Where(eventproposal.ID(id)).
		WithConversation().
		WithMessage().
		WithCalendarEvent().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, derr.ErrNotFound
		}
		return nil, err
	}
	return mapProposal(row), nil
}

// ListByConversation は会話の予定案を作成日時順で返す。
func (r *EventProposalRepository) ListByConversation(ctx context.Context, conversationID int) ([]*entity.EventProposal, error) {
	rows, err := r.client.EventProposal.Query().
		Where(eventproposal.HasConversationWith(conversation.IDEQ(conversationID))).
		WithConversation().
		WithMessage().
		WithCalendarEvent().
		Order(ent.Asc(eventproposal.FieldCreatedAt), ent.Asc(eventproposal.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*entity.EventProposal, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapProposal(row))
	}
	return out, nil
}

// EditableFields は承認時にユーザー編集を反映するためのフィールド。
type EditableFields struct {
	Title       string
	StartsAt    time.Time
	EndsAt      time.Time
	AllDay      bool
	Location    string
	Description string
	SourceURL   string
}

// MarkApproved は予定案を approved に更新し、作成された CalendarEvent を紐付ける。
// 編集後フィールドがある場合は予定案本体にも反映する。
func (r *EventProposalRepository) MarkApproved(ctx context.Context, id, calendarEventID int, edited *EditableFields) (*entity.EventProposal, error) {
	builder := r.client.EventProposal.UpdateOneID(id).
		SetStatus(eventproposal.StatusApproved).
		SetCalendarEventID(calendarEventID)
	if edited != nil {
		builder = builder.
			SetTitle(edited.Title).
			SetStartsAt(edited.StartsAt.UTC()).
			SetEndsAt(edited.EndsAt.UTC()).
			SetAllDay(edited.AllDay).
			SetLocation(edited.Location).
			SetDescription(edited.Description).
			SetSourceURL(edited.SourceURL)
	}
	if _, err := builder.Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return nil, derr.ErrNotFound
		}
		return nil, err
	}
	return r.Get(ctx, id)
}

// MarkRejected は予定案を rejected に更新する。
func (r *EventProposalRepository) MarkRejected(ctx context.Context, id int) (*entity.EventProposal, error) {
	if _, err := r.client.EventProposal.UpdateOneID(id).
		SetStatus(eventproposal.StatusRejected).
		Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return nil, derr.ErrNotFound
		}
		return nil, err
	}
	return r.Get(ctx, id)
}

// DeleteByConversation は会話に属する全予定案を削除する。
func (r *EventProposalRepository) DeleteByConversation(ctx context.Context, conversationID int) error {
	_, err := r.client.EventProposal.Delete().
		Where(eventproposal.HasConversationWith(conversation.IDEQ(conversationID))).
		Exec(ctx)
	return err
}

func mapProposal(row *ent.EventProposal) *entity.EventProposal {
	p := &entity.EventProposal{
		ID:          row.ID,
		Title:       row.Title,
		StartsAt:    row.StartsAt.UTC(),
		EndsAt:      row.EndsAt.UTC(),
		AllDay:      row.AllDay,
		Location:    row.Location,
		Description: row.Description,
		SourceURL:   row.SourceURL,
		Status:      entity.ProposalStatus(row.Status),
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
	if row.Edges.Conversation != nil {
		p.ConversationID = row.Edges.Conversation.ID
	}
	if row.Edges.Message != nil {
		id := row.Edges.Message.ID
		p.MessageID = &id
	}
	if row.Edges.CalendarEvent != nil {
		id := row.Edges.CalendarEvent.ID
		p.CalendarEventID = &id
	}
	return p
}
