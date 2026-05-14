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

type PipelineHandler struct {
	pipeline service.PipelineService
	log      *zap.Logger
}

func (h *PipelineHandler) List(c *gin.Context) {
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
	projectID, err := strconv.ParseInt(c.Query("project_id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	isTemplate, err := strconv.Atoi(c.Query("is_template"))
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	enabled, err := strconv.Atoi(c.Query("enabled"))
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}

	q := &repo.ListPipelinesQuery{
		Page:       page,
		PageSize:   pageSize,
		ProjectID:  projectID,
		IsTemplate: int8(isTemplate),
		Enabled:    int8(enabled),
	}
	list, total, err := h.pipeline.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

func (h *PipelineHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	v, err := h.pipeline.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *PipelineHandler) Create(c *gin.Context) {
	var in service.CreatePipelineInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	v, err := h.pipeline.Create(c.Request.Context(), &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *PipelineHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdatePipelineInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	v, err := h.pipeline.Update(c.Request.Context(), id, &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *PipelineHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.pipeline.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *PipelineHandler) ListRuns(c *gin.Context) {
	pipelineID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
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

	list, total, err := h.pipeline.ListRuns(c.Request.Context(), pipelineID, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

func (h *PipelineHandler) GetRun(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	v, err := h.pipeline.GetRun(c.Request.Context(), runID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *PipelineHandler) ListSteps(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.pipeline.ListSteps(c.Request.Context(), runID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

type runPipelineReq struct {
	Input         map[string]any `json:"input"`
	NodeOverrides map[string]any `json:"node_overrides"`
}

func (h *PipelineHandler) Run(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var req runPipelineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	runID, err := h.pipeline.Run(c.Request.Context(), id, req.Input, req.NodeOverrides)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"run_id": runID, "status": "queued"})
}
