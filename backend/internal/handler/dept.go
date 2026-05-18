package handler

import (
	"strconv"

	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DeptHandler struct {
	dept service.DeptService
	log  *zap.Logger
}

func (h *DeptHandler) List(c *gin.Context) {
	list, err := h.dept.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *DeptHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	d, err := h.dept.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	response.OK(c, d)
}

func (h *DeptHandler) Create(c *gin.Context) {
	var in service.CreateDeptInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	d, err := h.dept.Create(c.Request.Context(), &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, d)
}

type updateDeptReq struct {
	Name   string `json:"name"`
	Sort   int    `json:"sort"`
	Status int8   `json:"status"`
}

func (h *DeptHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var req updateDeptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	d, err := h.dept.Update(c.Request.Context(), id, req.Name, req.Sort, req.Status)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, d)
}

func (h *DeptHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.dept.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
