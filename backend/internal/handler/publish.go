package handler

import (
	"encoding/json"
	"strconv"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type PublishHandler struct {
	publish service.PublishService
	log     *zap.Logger
}

func (h *PublishHandler) Publish(c *gin.Context) {
	var in service.PublishInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	p, err := h.publish.Publish(c.Request.Context(), &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}

func (h *PublishHandler) Unpublish(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.publish.Unpublish(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *PublishHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	p, err := h.publish.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}

func (h *PublishHandler) List(c *gin.Context) {
	status := c.Query("status")
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
	list, total, err := h.publish.List(c.Request.Context(), status, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: page, PageSize: pageSize, Total: total})
}

func (h *PublishHandler) IncPlay(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.publish.IncPlay(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *PublishHandler) IncDownload(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.publish.IncDownload(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *PublishHandler) UpdateWatermark(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var body struct {
		WatermarkConfig json.RawMessage `json:"watermark_config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	p, err := h.publish.UpdateWatermark(c.Request.Context(), id, body.WatermarkConfig)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}
