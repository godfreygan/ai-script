// Sprint 3 - 分镜 / 风格 / 图片 / 短视频 / 上传 / 调用日志 handler
package handler

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// =================== Storyboard ===================

func (h *StoryboardHandler) ListByEpisode(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.story.ListByEpisode(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *StoryboardHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	sb, err := h.story.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	response.OK(c, sb)
}

type storyboardUpdateReq struct {
	ShotType     *string  `json:"shot_type"`
	CameraMotion *string  `json:"camera_motion"`
	SceneDesc    *string  `json:"scene_desc"`
	Characters   []string `json:"characters"`
	Action       *string  `json:"action"`
	Dialogue     *string  `json:"dialogue"`
	DurationSec  *int     `json:"duration_sec"`
	Notes        *string  `json:"notes"`
	Status       *int8    `json:"status"`
}

func (h *StoryboardHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req storyboardUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	sb, err := h.story.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	if req.ShotType != nil {
		sb.ShotType = *req.ShotType
	}
	if req.CameraMotion != nil {
		sb.CameraMotion = *req.CameraMotion
	}
	if req.SceneDesc != nil {
		sb.SceneDesc = *req.SceneDesc
	}
	if req.Characters != nil {
		b, _ := json.Marshal(req.Characters)
		sb.Characters = model.JSON(b)
	}
	if req.Action != nil {
		sb.Action = *req.Action
	}
	if req.Dialogue != nil {
		sb.Dialogue = *req.Dialogue
	}
	if req.DurationSec != nil {
		sb.DurationSec = *req.DurationSec
	}
	if req.Notes != nil {
		sb.Notes = *req.Notes
	}
	if req.Status != nil {
		sb.Status = *req.Status
	}
	if err := h.story.Update(c.Request.Context(), sb); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sb)
}

func (h *StoryboardHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.story.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

type storyboardBulkSaveReq struct {
	Shots []model.Storyboard `json:"shots"`
}

func (h *StoryboardHandler) BulkSave(c *gin.Context) {
	eid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req storyboardBulkSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.story.BulkSave(c.Request.Context(), eid, req.Shots); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"saved": len(req.Shots)})
}

type storyboardGenReq struct {
	ModelID int64          `json:"model_id" binding:"required"`
	Params  map[string]any `json:"params"`
}

func (h *StoryboardHandler) Generate(c *gin.Context) {
	eid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req storyboardGenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	taskID, err := h.story.Generate(c.Request.Context(), eid, req.ModelID, req.Params, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"task_id": taskID, "topic": "episode:" + strconv.FormatInt(eid, 10)})
}

type applyStyleReq struct {
	StyleID int64 `json:"style_id" binding:"required"`
}

func (h *StoryboardHandler) ApplyStyle(c *gin.Context) {
	sid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req applyStyleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	if err := h.story.ApplyStyle(c.Request.Context(), sid, req.StyleID, uid); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// =================== Style ===================

type StyleHandler struct {
	style *service.StyleService
	log   *zap.Logger
}

func (h *StyleHandler) List(c *gin.Context) {
	pid, _ := strconv.ParseInt(c.DefaultQuery("project_id", "0"), 10, 64)
	list, err := h.style.List(c.Request.Context(), pid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *StyleHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.style.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// =================== Image (列表 / 详情 / 删除) ===================

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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	img, err := h.img.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	response.OK(c, img)
}

func (h *ImageHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.img.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// =================== Short Video ===================

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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.short.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// =================== Upload ===================

type UploadHandler struct {
	upload *service.UploadService
	log    *zap.Logger
}

func (h *UploadHandler) Upload(c *gin.Context) {
	ns := strings.TrimSpace(c.DefaultPostForm("namespace", "misc"))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	defer f.Close()
	res, err := h.upload.Save(c.Request.Context(), ns, fileHeader.Filename, fileHeader.Header.Get("Content-Type"), io.Reader(f), fileHeader.Size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// =================== Invocation ===================

type InvocationHandler struct {
	invoke *service.InvocationService
	log    *zap.Logger
}

func (h *InvocationHandler) List(c *gin.Context) {
	q := &repo.ListInvocationsQuery{}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if s := c.Query("user_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.UserID = v
	}
	if s := c.Query("dept_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.DeptID = v
	}
	if s := c.Query("project_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.ProjectID = v
	}
	if s := c.Query("model_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.ModelID = v
	}
	q.BizType = c.Query("biz_type")
	q.Status = c.Query("status")
	if s := c.Query("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.From = &t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.To = &t
		}
	}
	list, total, err := h.invoke.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: q.Page, PageSize: q.PageSize, Total: total})
}

func (h *InvocationHandler) Stats(c *gin.Context) {
	q := &repo.ListInvocationsQuery{}
	if s := c.Query("user_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.UserID = v
	}
	if s := c.Query("dept_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.DeptID = v
	}
	if s := c.Query("project_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.ProjectID = v
	}
	if s := c.Query("model_id"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		q.ModelID = v
	}
	q.BizType = c.Query("biz_type")
	q.Status = c.Query("status")
	if s := c.Query("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.From = &t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.To = &t
		}
	}
	st, err := h.invoke.Stats(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, st)
}
