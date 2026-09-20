package chat

import (
	"context"
	"log/slog"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
)

// RejectProposal は pending の予定案を却下する（カレンダー登録は行わない）。
// 処理済みの予定案は却下できない。
func (u *UseCase) RejectProposal(ctx context.Context, proposalID int) (res *entity.EventProposal, err error) {
	defer logging.Trace(ctx, u.logger, "chat.UseCase.RejectProposal",
		logging.Args{"proposalID": proposalID}, &res, &err)()

	propRepo := persistence.NewEventProposalRepository(u.client, u.logger)

	proposal, err := propRepo.Get(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal.Status != entity.ProposalStatusPending {
		return nil, derr.ErrConflict
	}

	rejected, err := propRepo.MarkRejected(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	logging.LogContext(ctx, u.logger, slog.LevelInfo, "chat.proposal_rejected", slog.Int("proposal_id", proposalID))
	return rejected, nil
}
