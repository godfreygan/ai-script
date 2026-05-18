package handler

import (
	"github.com/godfreygan/ai-script/backend/pkg/ws"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type WSHandler struct {
	hub *ws.Hub
	log *zap.Logger
}

func (h *WSHandler) Progress(c *gin.Context) {
	topic := c.Query("topic")
	if err := h.hub.ServeWS(c.Writer, c.Request, topic); err != nil {
		h.log.Warn("ws upgrade failed", zap.Error(err))
	}
}
