package handler

import (
	"strconv"

	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ModelHandler struct {
	model service.ModelService
	log   *zap.Logger
}

func (h *ModelHandler) List(c *gin.Context) {
	q := &repo.ListModelsQuery{}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	q.Q = c.Query("q")
	q.Type = c.Query("type")
	q.Provider = c.Query("provider")
	if s := c.Query("enabled"); s != "" {
		v, _ := strconv.Atoi(s)
		q.Enabled = int8(v)
	}
	list, total, err := h.model.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: q.Page, PageSize: q.PageSize, Total: total})
}

func (h *ModelHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	m, err := h.model.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, m)
}

func (h *ModelHandler) Create(c *gin.Context) {
	var in service.CreateModelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, err)
		return
	}
	m, err := h.model.Create(c.Request.Context(), &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, m)
}

func (h *ModelHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdateModelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, err)
		return
	}
	m, err := h.model.Update(c.Request.Context(), id, &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, m)
}

func (h *ModelHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.model.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *ModelHandler) Healthcheck(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	ok, err := h.model.Healthcheck(c.Request.Context(), id)
	if err != nil {
		response.OK(c, gin.H{"healthy": false, "error": err.Error()})
		return
	}
	response.OK(c, gin.H{"healthy": ok})
}
