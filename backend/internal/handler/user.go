package handler

import (
	"strconv"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserHandler struct {
	user service.UserService
	auth service.AuthService
	log  *zap.Logger
}

func (h *UserHandler) Me(c *gin.Context) {
	uid := c.GetInt64("uid")
	u, err := h.user.Me(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, u)
}

func (h *UserHandler) List(c *gin.Context) {
	q := &repo.ListUsersQuery{}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	q.Q = c.Query("q")
	if s := c.Query("dept_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.DeptID = v
	}
	if s := c.Query("status"); s != "" {
		v, _ := strconv.Atoi(s)
		q.Status = int8(v)
	}
	list, total, err := h.user.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: q.Page, PageSize: q.PageSize, Total: total})
}

func (h *UserHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	u, err := h.user.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, u)
}

func (h *UserHandler) Create(c *gin.Context) {
	var in service.CreateUserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	u, err := h.user.Create(c.Request.Context(), &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, u)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdateUserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	u, err := h.user.Update(c.Request.Context(), id, &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, u)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.user.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

type resetPwReq struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var req resetPwReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.user.ResetPassword(c.Request.Context(), id, req.NewPassword); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

type changePwReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req changePwReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	if err := h.auth.ChangePassword(c.Request.Context(), uid, req.OldPassword, req.NewPassword); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
