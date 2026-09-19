package handler

import (
	"context"
	"net/http"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	chatuc "github.com/ITK13201/CalendarRabbit/backend/internal/usecase/chat"
	"github.com/gin-gonic/gin"
)

// ChatUseCase はチャットハンドラが依存するユースケース。
type ChatUseCase interface {
	SendMessage(ctx context.Context, content string) (*chatuc.SendResult, error)
	GetConversation(ctx context.Context) (*chatuc.ConversationView, error)
	ClearConversation(ctx context.Context) error
	ApproveProposal(ctx context.Context, proposalID int, edited *chatuc.ProposalEdit) (*entity.CalendarEvent, error)
	RejectProposal(ctx context.Context, proposalID int) (*entity.EventProposal, error)
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

type messageResponse struct {
	ID        int    `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type proposalResponse struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	StartsAt        string `json:"starts_at"`
	EndsAt          string `json:"ends_at"`
	AllDay          bool   `json:"all_day"`
	Location        string `json:"location"`
	Description     string `json:"description"`
	SourceURL       string `json:"source_url"`
	Status          string `json:"status"`
	CalendarEventID *int   `json:"calendar_event_id,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type candidateResponse struct {
	Title       string `json:"title"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	AllDay      bool   `json:"all_day"`
	Location    string `json:"location"`
	Description string `json:"description"`
	SourceURL   string `json:"source_url"`
}

type sendMessageResponse struct {
	UserMessage      messageResponse     `json:"user_message"`
	AssistantMessage messageResponse     `json:"assistant_message"`
	Proposal         *proposalResponse   `json:"proposal,omitempty"`
	Candidates       []candidateResponse `json:"candidates,omitempty"`
}

type conversationResponse struct {
	ID        int                `json:"id"`
	Messages  []messageResponse  `json:"messages"`
	Proposals []proposalResponse `json:"proposals"`
}

type approveRequest struct {
	Edited *eventRequest `json:"edited"`
}

func toMessageResponse(m *entity.Message) messageResponse {
	return messageResponse{
		ID:        m.ID,
		Role:      string(m.Role),
		Content:   m.Content,
		CreatedAt: m.CreatedAt.UTC().Format(timeRFC3339),
	}
}

func toProposalResponse(p *entity.EventProposal) proposalResponse {
	return proposalResponse{
		ID:              p.ID,
		Title:           p.Title,
		StartsAt:        p.StartsAt.UTC().Format(timeRFC3339),
		EndsAt:          p.EndsAt.UTC().Format(timeRFC3339),
		AllDay:          p.AllDay,
		Location:        p.Location,
		Description:     p.Description,
		SourceURL:       p.SourceURL,
		Status:          string(p.Status),
		CalendarEventID: p.CalendarEventID,
		CreatedAt:       p.CreatedAt.UTC().Format(timeRFC3339),
		UpdatedAt:       p.UpdatedAt.UTC().Format(timeRFC3339),
	}
}

// SendMessage godoc
// @Summary  チャットメッセージを送信する
// @Tags     chat
// @Accept   json
// @Produce  json
// @Param    body body sendMessageRequest true "メッセージ"
// @Success  200 {object} sendMessageResponse
// @Failure  400 {object} errorResponse
// @Router   /api/chat/messages [post]
func (h *Handler) SendMessage(c *gin.Context) {
	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, badRequest("body", "invalid request body"))
		return
	}
	if req.Content == "" {
		respondError(c, badRequest("content", "content is required"))
		return
	}
	res, err := h.chat.SendMessage(c.Request.Context(), req.Content)
	if err != nil {
		respondError(c, err)
		return
	}

	resp := sendMessageResponse{
		UserMessage:      toMessageResponse(res.UserMessage),
		AssistantMessage: toMessageResponse(res.AssistantMessage),
	}
	if res.Proposal != nil {
		p := toProposalResponse(res.Proposal)
		resp.Proposal = &p
	}
	for _, cand := range res.Candidates {
		resp.Candidates = append(resp.Candidates, candidateResponse{
			Title:       cand.Title,
			StartsAt:    cand.StartsAt.UTC().Format(timeRFC3339),
			EndsAt:      cand.EndsAt.UTC().Format(timeRFC3339),
			AllDay:      cand.AllDay,
			Location:    cand.Location,
			Description: cand.Description,
			SourceURL:   cand.SourceURL,
		})
	}
	c.JSON(http.StatusOK, resp)
}

// GetConversation godoc
// @Summary  会話履歴を取得する（単一連続スレッド）
// @Tags     chat
// @Produce  json
// @Success  200 {object} conversationResponse
// @Router   /api/chat/conversations [get]
func (h *Handler) GetConversation(c *gin.Context) {
	view, err := h.chat.GetConversation(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	resp := conversationResponse{
		ID:        view.Conversation.ID,
		Messages:  make([]messageResponse, 0, len(view.Messages)),
		Proposals: make([]proposalResponse, 0, len(view.Proposals)),
	}
	for _, m := range view.Messages {
		resp.Messages = append(resp.Messages, toMessageResponse(m))
	}
	for _, p := range view.Proposals {
		resp.Proposals = append(resp.Proposals, toProposalResponse(p))
	}
	c.JSON(http.StatusOK, resp)
}

// ClearConversation godoc
// @Summary  会話履歴をクリアする（全メッセージ・予定案を削除）
// @Tags     chat
// @Success  204 "No Content"
// @Failure  500 {object} errorResponse
// @Router   /api/chat/conversations [delete]
func (h *Handler) ClearConversation(c *gin.Context) {
	if err := h.chat.ClearConversation(c.Request.Context()); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ApproveProposal godoc
// @Summary  予定案を承認してカレンダーに登録する
// @Tags     chat
// @Accept   json
// @Produce  json
// @Param    id path int true "予定案ID"
// @Param    body body approveRequest false "編集後内容(任意)"
// @Success  200 {object} eventResponse
// @Failure  400 {object} errorResponse
// @Failure  404 {object} errorResponse
// @Failure  409 {object} errorResponse
// @Router   /api/chat/proposals/{id}/approve [post]
func (h *Handler) ApproveProposal(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respondError(c, err)
		return
	}

	var req approveRequest
	// body は任意。存在しない/不正でも編集なしとして扱う。
	_ = c.ShouldBindJSON(&req)

	var edited *chatuc.ProposalEdit
	if req.Edited != nil {
		start, perr := parseTime("starts_at", req.Edited.StartsAt, true)
		if perr != nil {
			respondError(c, perr)
			return
		}
		end, perr := parseTime("ends_at", req.Edited.EndsAt, true)
		if perr != nil {
			respondError(c, perr)
			return
		}
		edited = &chatuc.ProposalEdit{
			Title:       req.Edited.Title,
			StartsAt:    start,
			EndsAt:      end,
			AllDay:      req.Edited.AllDay,
			Location:    req.Edited.Location,
			Description: req.Edited.Description,
			SourceURL:   req.Edited.SourceURL,
		}
	}

	event, err := h.chat.ApproveProposal(c.Request.Context(), id, edited)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toEventResponse(event))
}

// RejectProposal godoc
// @Summary  予定案を却下する
// @Tags     chat
// @Produce  json
// @Param    id path int true "予定案ID"
// @Success  200 {object} proposalResponse
// @Failure  404 {object} errorResponse
// @Failure  409 {object} errorResponse
// @Router   /api/chat/proposals/{id}/reject [post]
func (h *Handler) RejectProposal(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respondError(c, err)
		return
	}
	proposal, err := h.chat.RejectProposal(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProposalResponse(proposal))
}
