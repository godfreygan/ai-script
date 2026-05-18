package handler

import (
	"strconv"

	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type StyleHandler struct {
	style service.StyleService
	log   *zap.Logger
}

func (h *StyleHandler) List(c *gin.Context) {
	pid, err := strconv.ParseInt(c.DefaultQuery("project_id", "0"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.style.List(c.Request.Context(), pid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *StyleHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	st, err := h.style.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	response.OK(c, st)
}

func (h *StyleHandler) Create(c *gin.Context) {
	var in service.CreateStyleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	st, err := h.style.Create(c.Request.Context(), &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, st)
}

func (h *StyleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdateStyleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	st, err := h.style.Update(c.Request.Context(), id, &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, st)
}

func (h *StyleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.style.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
