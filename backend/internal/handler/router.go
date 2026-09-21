package handler

import (
	"log/slog"

	"github.com/ITK13201/CalendarRabbit/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RouterConfig はルータ構築の設定。
type RouterConfig struct {
	Logger         *slog.Logger
	AllowedOrigins []string
}

// NewRouter は共通ミドルウェアとルートを構成した gin エンジンを返す。
func NewRouter(h *Handler, cfg RouterConfig) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Logger(cfg.Logger))
	engine.Use(middleware.CORS(cfg.AllowedOrigins))

	h.registerRoutes(engine)
	return engine
}

// registerRoutes は API ルートを登録する。
func (h *Handler) registerRoutes(engine *gin.Engine) {
	api := engine.Group("/api")
	api.GET("/health", h.Health)

	events := api.Group("/calendar/events")
	{
		events.POST("", h.CreateEvent)
		events.GET("", h.ListEvents)
		events.GET("/:id", h.GetEvent)
		events.PUT("/:id", h.UpdateEvent)
		events.DELETE("/:id", h.DeleteEvent)
	}

	settings := api.Group("/settings")
	{
		settings.GET("", h.GetSettings)
		settings.PUT("", h.UpdateSettings)
	}

	chat := api.Group("/chat")
	{
		chat.POST("/messages", h.SendMessage)
		chat.GET("/conversations", h.GetConversation)
		chat.DELETE("/conversations", h.ClearConversation)
		chat.POST("/proposals/:id/approve", h.ApproveProposal)
		chat.POST("/proposals/:id/reject", h.RejectProposal)
	}

	google := api.Group("/google")
	{
		google.GET("/auth", h.GoogleAuth)
		google.GET("/callback", h.GoogleCallback)
		google.GET("/status", h.GoogleStatus)
		google.DELETE("/connection", h.GoogleDisconnect)
		google.POST("/resync", h.GoogleResync)
	}
}
