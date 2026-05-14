package handler

import (
	"strconv"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type InvocationHandler struct {
	invoke service.InvocationService
	log    *zap.Logger
}

func (h *InvocationHandler) List(c *gin.Context) {
	q := &repo.ListInvocationsQuery{}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if s := c.Query("user_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.UserID = v
	}
	if s := c.Query("dept_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.DeptID = v
	}
	if s := c.Query("project_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.ProjectID = v
	}
	if s := c.Query("model_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.ModelID = v
	}
	q.BizType = c.Query("biz_type")
	q.Status = c.Query("status")
	if s := c.Query("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.From = &t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.To = &t
		}
	}
	list, total, err := h.invoke.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: q.Page, PageSize: q.PageSize, Total: total})
}

func (h *InvocationHandler) Stats(c *gin.Context) {
	q := &repo.ListInvocationsQuery{}
	if s := c.Query("user_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.UserID = v
	}
	if s := c.Query("dept_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.DeptID = v
	}
	if s := c.Query("project_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.ProjectID = v
	}
	if s := c.Query("model_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.ModelID = v
	}
	q.BizType = c.Query("biz_type")
	q.Status = c.Query("status")
	if s := c.Query("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.From = &t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.To = &t
		}
	}
	st, err := h.invoke.Stats(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, st)
}
