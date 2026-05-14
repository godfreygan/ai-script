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

type ProjectHandler struct {
	project service.ProjectService
	log     *zap.Logger
}

func (h *ProjectHandler) List(c *gin.Context) {
	q := &repo.ListProjectsQuery{}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if s := c.Query("status"); s != "" {
		v, _ := strconv.Atoi(s)
		q.Status = int8(v)
	}
	q.Q = c.Query("q")
	q.UserID = c.GetInt64("uid")
	q.DeptID = c.GetInt64("dept_id")
	if rs, ok := c.Get("roles"); ok {
		if codes, ok := rs.([]string); ok {
			q.DataScope = scopeFromRoles(codes)
		}
	}

	list, total, err := h.project.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: q.Page, PageSize: q.PageSize, Total: total})
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var in service.CreateProjectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	deptID := c.GetInt64("dept_id")
	p, err := h.project.Create(c.Request.Context(), &in, uid, deptID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}

func (h *ProjectHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	p, err := h.project.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	response.OK(c, p)
}

func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdateProjectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	p, err := h.project.Update(c.Request.Context(), id, &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.project.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *ProjectHandler) ListMembers(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.project.ListMembers(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

type addMemberReq struct {
	UserID        int64  `json:"user_id" binding:"required"`
	RoleInProject string `json:"role_in_project"`
}

func (h *ProjectHandler) AddMember(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var req addMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.project.AddMember(c.Request.Context(), id, req.UserID, req.RoleInProject); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *ProjectHandler) RemoveMember(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.project.RemoveMember(c.Request.Context(), id, uid); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
