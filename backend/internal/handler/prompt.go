package handler

import (
	"strconv"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type PromptHandler struct {
	prompt service.PromptService
	log    *zap.Logger
}

func (h *PromptHandler) ListByEpisode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.prompt.ListByEpisode(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *PromptHandler) GetCurrent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	p, err := h.prompt.GetCurrent(c.Request.Context(), id)
	if err != nil {
		response.OK(c, nil)
		return
	}
	response.OK(c, p)
}

func (h *PromptHandler) Generate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.GeneratePromptInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid, _ := c.Get("uid")
	taskID, err := h.prompt.Generate(c.Request.Context(), id, &in, asInt64(uid))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"task_id": taskID,
		"topic":   "episode:" + strconv.FormatInt(id, 10),
	})
}

func (h *PromptHandler) SetCurrent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var body struct {
		EpisodeID int64 `json:"episode_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.prompt.SetCurrent(c.Request.Context(), body.EpisodeID, id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}
