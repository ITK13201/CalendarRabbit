package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/derr"
	"github.com/ITK13201/CalendarRabbit/backend/internal/domain/entity"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/calendar"
	"github.com/gin-gonic/gin"
)

// CalendarUseCase はカレンダーハンドラが依存するユースケース。
type CalendarUseCase interface {
	Create(ctx context.Context, in calendar.EventInput) (*entity.CalendarEvent, error)
	Update(ctx context.Context, id int, in calendar.EventInput) (*entity.CalendarEvent, error)
	Delete(ctx context.Context, id int) error
	Get(ctx context.Context, id int) (*entity.CalendarEvent, error)
	List(ctx context.Context) ([]*entity.CalendarEvent, error)
	ListByPeriod(ctx context.Context, from, to time.Time) ([]*entity.CalendarEvent, error)
}

// eventRequest はイベント作成・更新のリクエスト。
type eventRequest struct {
	Title       string `json:"title"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	AllDay      bool   `json:"all_day"`
	Location    string `json:"location"`
	Description string `json:"description"`
	SourceURL   string `json:"source_url"`
}

// eventResponse はイベントのレスポンス。
type eventResponse struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	AllDay      bool   `json:"all_day"`
	Location    string `json:"location"`
	Description string `json:"description"`
	SourceURL   string `json:"source_url"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toEventResponse(e *entity.CalendarEvent) eventResponse {
	return eventResponse{
		ID:          e.ID,
		Title:       e.Title,
		StartsAt:    e.StartsAt.UTC().Format(time.RFC3339),
		EndsAt:      e.EndsAt.UTC().Format(time.RFC3339),
		AllDay:      e.AllDay,
		Location:    e.Location,
		Description: e.Description,
		SourceURL:   e.SourceURL,
		CreatedAt:   e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   e.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (r eventRequest) toInput() (calendar.EventInput, error) {
	start, err := parseTime("starts_at", r.StartsAt, true)
	if err != nil {
		return calendar.EventInput{}, err
	}
	end, err := parseTime("ends_at", r.EndsAt, true)
	if err != nil {
		return calendar.EventInput{}, err
	}
	return calendar.EventInput{
		Title:       r.Title,
		StartsAt:    start,
		EndsAt:      end,
		AllDay:      r.AllDay,
		Location:    r.Location,
		Description: r.Description,
		SourceURL:   r.SourceURL,
	}, nil
}

// parseTime は RFC3339 文字列を time.Time に変換する。required=false で空を許容。
func parseTime(field, raw string, required bool) (time.Time, error) {
	if raw == "" {
		if required {
			return time.Time{}, derr.NewValidationError(field, field+" is required")
		}
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, derr.NewValidationError(field, field+" must be RFC3339")
	}
	return t.UTC(), nil
}

// CreateEvent godoc
// @Summary  イベントを作成する
// @Tags     calendar
// @Accept   json
// @Produce  json
// @Param    body body eventRequest true "イベント"
// @Success  201 {object} eventResponse
// @Failure  400 {object} errorResponse
// @Router   /api/calendar/events [post]
func (h *Handler) CreateEvent(c *gin.Context) {
	var req eventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, derr.NewValidationError("body", "invalid request body"))
		return
	}
	in, err := req.toInput()
	if err != nil {
		respondError(c, err)
		return
	}
	event, err := h.calendar.Create(c.Request.Context(), in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toEventResponse(event))
}

// ListEvents godoc
// @Summary  イベント一覧を取得する（from/to 指定で期間取得）
// @Tags     calendar
// @Produce  json
// @Param    from query string false "期間開始(RFC3339)"
// @Param    to query string false "期間終了(RFC3339)"
// @Success  200 {array} eventResponse
// @Failure  400 {object} errorResponse
// @Router   /api/calendar/events [get]
func (h *Handler) ListEvents(c *gin.Context) {
	fromRaw := c.Query("from")
	toRaw := c.Query("to")

	var (
		events []*entity.CalendarEvent
		err    error
	)
	if fromRaw != "" || toRaw != "" {
		from, perr := parseTime("from", fromRaw, true)
		if perr != nil {
			respondError(c, perr)
			return
		}
		to, perr := parseTime("to", toRaw, true)
		if perr != nil {
			respondError(c, perr)
			return
		}
		events, err = h.calendar.ListByPeriod(c.Request.Context(), from, to)
	} else {
		events, err = h.calendar.List(c.Request.Context())
	}
	if err != nil {
		respondError(c, err)
		return
	}
	resp := make([]eventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, toEventResponse(e))
	}
	c.JSON(http.StatusOK, resp)
}

// GetEvent godoc
// @Summary  イベントを取得する
// @Tags     calendar
// @Produce  json
// @Param    id path int true "イベントID"
// @Success  200 {object} eventResponse
// @Failure  404 {object} errorResponse
// @Router   /api/calendar/events/{id} [get]
func (h *Handler) GetEvent(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respondError(c, err)
		return
	}
	event, err := h.calendar.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toEventResponse(event))
}

// UpdateEvent godoc
// @Summary  イベントを更新する
// @Tags     calendar
// @Accept   json
// @Produce  json
// @Param    id path int true "イベントID"
// @Param    body body eventRequest true "イベント"
// @Success  200 {object} eventResponse
// @Failure  400 {object} errorResponse
// @Failure  404 {object} errorResponse
// @Router   /api/calendar/events/{id} [put]
func (h *Handler) UpdateEvent(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respondError(c, err)
		return
	}
	var req eventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, derr.NewValidationError("body", "invalid request body"))
		return
	}
	in, err := req.toInput()
	if err != nil {
		respondError(c, err)
		return
	}
	event, err := h.calendar.Update(c.Request.Context(), id, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toEventResponse(event))
}

// DeleteEvent godoc
// @Summary  イベントを削除する
// @Tags     calendar
// @Param    id path int true "イベントID"
// @Success  204
// @Failure  404 {object} errorResponse
// @Router   /api/calendar/events/{id} [delete]
func (h *Handler) DeleteEvent(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respondError(c, err)
		return
	}
	if err := h.calendar.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// pathID は URL パスの :id を整数として取り出す。
func pathID(c *gin.Context) (int, error) {
	raw := c.Param("id")
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, derr.NewValidationError("id", "invalid id")
	}
	return id, nil
}
