package handler

import (
	"strconv"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
)

// ===================== FullVideoHandler =====================

func (h *FullVideoHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	projectID, _ := strconv.ParseInt(c.Query("project_id"), 10, 64)
	status := c.Query("status")

	q := &repo.ListFullVideosQuery{
		Page:      page,
		PageSize:  pageSize,
		ProjectID: projectID,
		Status:    status,
	}
	list, total, err := h.full.List(c.Request.Context(), q)
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

func (h *FullVideoHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	v, err := h.full.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *FullVideoHandler) Create(c *gin.Context) {
	var in service.CreateFullVideoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	v, err := h.full.Create(c.Request.Context(), &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *FullVideoHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdateFullVideoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	v, err := h.full.Update(c.Request.Context(), id, &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *FullVideoHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.full.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *FullVideoHandler) Render(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	taskID, err := h.full.Render(c.Request.Context(), id, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"task_id": taskID,
		"topic":   "full:" + strconv.FormatInt(id, 10),
	})
}

// ===================== PipelineHandler =====================

func (h *PipelineHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	projectID, _ := strconv.ParseInt(c.Query("project_id"), 10, 64)
	isTemplate, _ := strconv.Atoi(c.Query("is_template"))
	enabled, _ := strconv.Atoi(c.Query("enabled"))

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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

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
