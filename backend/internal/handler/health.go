package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health godoc
// @Summary  ヘルスチェック
// @Tags     health
// @Produce  json
// @Success  200 {object} map[string]string
// @Router   /api/health [get]
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
