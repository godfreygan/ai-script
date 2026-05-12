package handler

import (
	"strconv"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/ws"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// =============== Script ===============

// 注意:ScriptHandler 类型本身已在 handlers.go 中定义,这里只补方法

func (h *ScriptHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pid, _ := strconv.ParseInt(c.Query("project_id"), 10, 64)
	st, _ := strconv.Atoi(c.Query("status"))
	q := &repo.ListScriptsQuery{
		Page: page, PageSize: pageSize,
		ProjectID: pid, Status: int8(st),
		Q: c.Query("q"),
	}
	list, total, err := h.script.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

func (h *ScriptHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	sc, err := h.script.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sc)
}

func (h *ScriptHandler) Create(c *gin.Context) {
	var in service.CreateScriptInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid, _ := c.Get("uid")
	sc, err := h.script.Create(c.Request.Context(), &in, asInt64(uid))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sc)
}

func (h *ScriptHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.script.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *ScriptHandler) ListEpisodes(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.script.ListEpisodes(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *ScriptHandler) Split(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in service.SplitScriptInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid, _ := c.Get("uid")
	taskID, err := h.script.Split(c.Request.Context(), id, &in, asInt64(uid))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"task_id": taskID,
		"topic":   "script:" + strconv.FormatInt(id, 10),
	})
}

// =============== Prompt ===============

func (h *PromptHandler) ListByEpisode(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.prompt.ListByEpisode(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *PromptHandler) GetCurrent(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	p, err := h.prompt.GetCurrent(c.Request.Context(), id)
	if err != nil {
		response.OK(c, nil)
		return
	}
	response.OK(c, p)
}

func (h *PromptHandler) Generate(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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

// =============== WebSocket ===============

type WSHandler struct {
	hub *ws.Hub
	log *zap.Logger
}

// Progress 升级为 WS,按 ?topic= 订阅
func (h *WSHandler) Progress(c *gin.Context) {
	topic := c.Query("topic")
	if err := h.hub.ServeWS(c.Writer, c.Request, topic); err != nil {
		h.log.Warn("ws upgrade failed", zap.Error(err))
	}
}

// =============== utils ===============

func asInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}
