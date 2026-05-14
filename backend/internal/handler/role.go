package handler

import (
	"strconv"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RoleHandler struct {
	role service.RoleService
	log  *zap.Logger
}

func (h *RoleHandler) List(c *gin.Context) {
	list, err := h.role.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *RoleHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	r, err := h.role.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, r)
}

func (h *RoleHandler) ListPermissions(c *gin.Context) {
	perms, err := h.role.ListPermissions(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, perms)
}

func (h *RoleHandler) Create(c *gin.Context) {
	var in service.CreateRoleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	r, err := h.role.Create(c.Request.Context(), &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, r)
}

func (h *RoleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdateRoleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	r, err := h.role.Update(c.Request.Context(), id, &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, r)
}

func (h *RoleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.role.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
