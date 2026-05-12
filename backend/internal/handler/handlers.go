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

// =================== Auth ===================

type AuthHandler struct {
	auth *service.AuthService
	log  *zap.Logger
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	r, err := h.auth.Login(c.Request.Context(), req.Username, req.Password, c.ClientIP())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, r)
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	r, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, r)
}

func (h *AuthHandler) Logout(c *gin.Context) { response.OK(c, nil) }

// =================== User ===================

type UserHandler struct {
	user *service.UserService
	auth *service.AuthService
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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

// =================== Dept ===================

type DeptHandler struct {
	dept *service.DeptService
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.dept.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// =================== Role ===================

type RoleHandler struct {
	role *service.RoleService
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.role.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// =================== Project ===================

type ProjectHandler struct {
	project *service.ProjectService
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
	// 数据范围由 caller 传入 roles 决定;Sprint 1 暂取 c.GetInt64("dept_id") + c.GetInt64("uid") 作为参考
	q.UserID = c.GetInt64("uid")
	q.DeptID = c.GetInt64("dept_id")
	// 根据角色决定 data_scope:含 super_admin -> all, 含 dept_admin -> dept, 其余 -> self
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

func scopeFromRoles(codes []string) string {
	for _, r := range codes {
		if r == "super_admin" {
			return "all"
		}
	}
	for _, r := range codes {
		if r == "dept_admin" {
			return "dept"
		}
	}
	return "self"
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	p, err := h.project.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	response.OK(c, p)
}

func (h *ProjectHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.project.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *ProjectHandler) ListMembers(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid, _ := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err := h.project.RemoveMember(c.Request.Context(), id, uid); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// =================== Model ===================

type ModelHandler struct {
	model *service.ModelService
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
		response.Fail(c, errcode.ErrParam.Wrap(err))
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in service.UpdateModelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.model.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *ModelHandler) Healthcheck(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ok, err := h.model.Healthcheck(c.Request.Context(), id)
	if err != nil {
		response.OK(c, gin.H{"healthy": false, "error": err.Error()})
		return
	}
	response.OK(c, gin.H{"healthy": ok})
}

// =================== Sprint 2+ 占位 handler ===================

type ScriptHandler struct {
	script *service.ScriptService
	log    *zap.Logger
}
type PromptHandler struct {
	prompt *service.PromptService
	log    *zap.Logger
}
type StoryboardHandler struct {
	story *service.StoryboardService
	log   *zap.Logger
}
type ImageHandler struct {
	img *service.ImageService
	log *zap.Logger
}
type ShortVideoHandler struct {
	short *service.ShortVideoService
	log   *zap.Logger
}
type FullVideoHandler struct {
	full *service.FullVideoService
	log  *zap.Logger
}
type PipelineHandler struct {
	pipeline *service.PipelineService
	log      *zap.Logger
}

// 简化示例:图片生成接口
func (h *ImageHandler) Generate(c *gin.Context) {
	var in service.ImageGenInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	deptID := c.GetInt64("dept_id")
	taskID, err := h.img.Generate(c.Request.Context(), &in, uid, deptID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"task_id": taskID, "topic": "image:" + strconv.FormatInt(in.StoryboardID, 10)})
}

type runPipelineReq struct {
	Input         map[string]any `json:"input"`
	NodeOverrides map[string]any `json:"node_overrides"`
}

func (h *PipelineHandler) Run(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req runPipelineReq
	_ = c.ShouldBindJSON(&req)
	runID, err := h.pipeline.Run(c.Request.Context(), id, req.Input, req.NodeOverrides)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"run_id": runID, "status": "queued"})
}
