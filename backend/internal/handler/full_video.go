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

type FullVideoHandler struct {
	full service.FullVideoService
	log  *zap.Logger
}

func (h *FullVideoHandler) List(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var projectID int64
	if s := c.Query("project_id"); s != "" {
		if projectID, err = strconv.ParseInt(s, 10, 64); err != nil {
			response.Fail(c, errcode.ErrParam.Wrap(err))
			return
		}
	}
	status := c.Query("status")

	q := &repo.ListFullVideosQuery{
		Page:      page,
		PageSize:  pageSize,
		ProjectID: projectID,
		Status:    status,
	}
	list, total, err := h.full.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

func (h *FullVideoHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	v, err := h.full.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *FullVideoHandler) Create(c *gin.Context) {
	var in service.CreateFullVideoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	v, err := h.full.Create(c.Request.Context(), &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *FullVideoHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdateFullVideoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	v, err := h.full.Update(c.Request.Context(), id, &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *FullVideoHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.full.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *FullVideoHandler) Render(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	taskID, err := h.full.Render(c.Request.Context(), id, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"task_id": taskID,
		"topic":   "full:" + strconv.FormatInt(id, 10),
	})
}
