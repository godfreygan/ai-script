package handler

import (
	"strconv"

	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuditHandler struct {
	audit service.AuditService
	log   *zap.Logger
}

func (h *AuditHandler) List(c *gin.Context) {
	q := &repo.ListAuditQuery{}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	q.UserID, _ = strconv.ParseInt(c.Query("user_id"), 10, 64)
	q.ResourceType = c.Query("resource_type")
	q.Action = c.Query("action")
	list, total, err := h.audit.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: q.Page, PageSize: q.PageSize, Total: total})
}
