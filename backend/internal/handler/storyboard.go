package handler

import (
	"encoding/json"
	"strconv"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type StoryboardHandler struct {
	story service.StoryboardService
	log   *zap.Logger
}

func (h *StoryboardHandler) ListByEpisode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.story.ListByEpisode(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *StoryboardHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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
	eid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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
	eid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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
	sid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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
