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

type ImageHandler struct {
	img service.ImageService
	log *zap.Logger
}

func (h *ImageHandler) List(c *gin.Context) {
	q := &repo.ListImagesQuery{}
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
	if s := c.Query("status"); s != "" {
		v, _ := strconv.Atoi(s)
		q.Status = int8(v)
	}
	list, total, err := h.img.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: q.Page, PageSize: q.PageSize, Total: total})
}

func (h *ImageHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	img, err := h.img.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	response.OK(c, img)
}

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

func (h *ImageHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.img.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
