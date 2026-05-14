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

type ShortVideoHandler struct {
	short service.ShortVideoService
	log   *zap.Logger
}

func (h *ShortVideoHandler) List(c *gin.Context) {
	q := &repo.ListShortVideosQuery{}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if s := c.Query("project_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.ProjectID = v
	}
	if s := c.Query("storyboard_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.StoryboardID = v
	}
	q.Status = c.Query("status")
	list, total, err := h.short.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: q.Page, PageSize: q.PageSize, Total: total})
}

func (h *ShortVideoHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	sv, err := h.short.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	response.OK(c, sv)
}

func (h *ShortVideoHandler) Generate(c *gin.Context) {
	var in service.ShortVideoGenInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	deptID := c.GetInt64("dept_id")
	taskID, err := h.short.Generate(c.Request.Context(), &in, uid, deptID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"task_id": taskID, "topic": "short:" + strconv.FormatInt(in.StoryboardID, 10)})
}

func (h *ShortVideoHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.short.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
