package chat

import (
	"context"
	"log/slog"

	"github.com/ITK13201/CalendarRabbit/backend/ent"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/calendarprovider"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
)

// ApproveProposal は pending の予定案を承認し、CalendarProvider 経由で
// イベントを作成、予定案を approved に更新する（トランザクション）。
// edited が指定された場合はその内容でイベントを作成する。
// 処理済み（approved/rejected）の予定案は再承認できない（design.md D2）。
func (u *UseCase) ApproveProposal(ctx context.Context, proposalID int, edited *ProposalEdit) (*entity.CalendarEvent, error) {
	var created *entity.CalendarEvent

	err := persistence.WithTx(ctx, u.client, func(txClient *ent.Client) error {
		propRepo := persistence.NewEventProposalRepository(txClient)
		provider := calendarprovider.NewDBProvider(persistence.NewCalendarEventRepository(txClient))

		proposal, err := propRepo.Get(ctx, proposalID)
		if err != nil {
			return err
		}
		if proposal.Status != entity.ProposalStatusPending {
			return derr.ErrConflict
		}

		input := calendarprovider.EventInput{
			Title:       proposal.Title,
			StartsAt:    proposal.StartsAt,
			EndsAt:      proposal.EndsAt,
			AllDay:      proposal.AllDay,
			Location:    proposal.Location,
			Description: proposal.Description,
			SourceURL:   proposal.SourceURL,
		}
		var editedFields *persistence.EditableFields
		if edited != nil {
			if err := validateEdit(*edited); err != nil {
				return err
			}
			input = calendarprovider.EventInput{
				Title:       edited.Title,
				StartsAt:    edited.StartsAt,
				EndsAt:      edited.EndsAt,
				AllDay:      edited.AllDay,
				Location:    edited.Location,
				Description: edited.Description,
				SourceURL:   edited.SourceURL,
			}
			editedFields = &persistence.EditableFields{
				Title:       edited.Title,
				StartsAt:    edited.StartsAt,
				EndsAt:      edited.EndsAt,
				AllDay:      edited.AllDay,
				Location:    edited.Location,
				Description: edited.Description,
				SourceURL:   edited.SourceURL,
			}
		}

		event, err := provider.Create(ctx, input)
		if err != nil {
			return err
		}
		if _, err := propRepo.MarkApproved(ctx, proposalID, event.ID, editedFields); err != nil {
			return err
		}
		created = event
		return nil
	})
	if err != nil {
		return nil, err
	}

	logging.LogContext(ctx, u.logger, slog.LevelInfo, "chat.proposal_approved",
		slog.Int("proposal_id", proposalID), slog.Int("event_id", created.ID))
	return created, nil
}

// validateEdit は編集後内容の妥当性を検証する。
func validateEdit(e ProposalEdit) error {
	if e.Title == "" {
		return derr.NewValidationError("title", "title is required")
	}
	if e.StartsAt.IsZero() {
		return derr.NewValidationError("starts_at", "starts_at is required")
	}
	if e.EndsAt.IsZero() {
		return derr.NewValidationError("ends_at", "ends_at is required")
	}
	if e.EndsAt.Before(e.StartsAt) {
		return derr.NewValidationError("ends_at", "ends_at must not be before starts_at")
	}
	return nil
}
