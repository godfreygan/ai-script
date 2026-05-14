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

type ScriptHandler struct {
	script service.ScriptService
	log    *zap.Logger
}

func (h *ScriptHandler) List(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	pid, err := strconv.ParseInt(c.Query("project_id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	st, err := strconv.Atoi(c.Query("status"))
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.script.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *ScriptHandler) ListEpisodes(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.script.ListEpisodes(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *ScriptHandler) Split(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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
