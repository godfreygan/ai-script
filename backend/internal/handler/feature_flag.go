package handler

import (
	"strconv"

	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/godfreygan/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type FeatureFlagHandler struct {
	flag service.FeatureFlagService
	log  *zap.Logger
}

func (h *FeatureFlagHandler) List(c *gin.Context) {
	list, err := h.flag.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *FeatureFlagHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	f, err := h.flag.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, f)
}

func (h *FeatureFlagHandler) Create(c *gin.Context) {
	var in service.CreateFlagInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid, _ := c.Get("uid")
	f, err := h.flag.Create(c.Request.Context(), &in, asInt64(uid))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, f)
}

func (h *FeatureFlagHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdateFlagInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid, _ := c.Get("uid")
	f, err := h.flag.Update(c.Request.Context(), id, &in, asInt64(uid))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, f)
}

func (h *FeatureFlagHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.flag.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *FeatureFlagHandler) Evaluate(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		response.Fail(c, errcode.ErrParam)
		return
	}
	uid, _ := c.Get("uid")
	deptID, _ := c.Get("dept_id")
	projectID, _ := c.Get("project_id")
	fc := &service.FlagContext{
		UserID:    asInt64(uid),
		DeptID:    asInt64(deptID),
		ProjectID: asInt64(projectID),
	}
	ok, err := h.flag.Evaluate(c.Request.Context(), key, fc)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"key": key, "enabled": ok})
}
