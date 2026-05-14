package handler

import (
	"strconv"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ReviewHandler struct {
	review service.ReviewService
	log    *zap.Logger
}

func (h *ReviewHandler) ListFlows(c *gin.Context) {
	list, err := h.review.ListFlows(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *ReviewHandler) GetFlow(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	f, err := h.review.GetFlow(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, f)
}

func (h *ReviewHandler) ListNodes(c *gin.Context) {
	flowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.review.ListNodes(c.Request.Context(), flowID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *ReviewHandler) Submit(c *gin.Context) {
	var in service.SubmitInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	rec, err := h.review.Submit(c.Request.Context(), &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rec)
}

func (h *ReviewHandler) ListRecords(c *gin.Context) {
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
	list, total, err := h.review.ListRecords(c.Request.Context(), status, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: page, PageSize: pageSize, Total: total})
}

func (h *ReviewHandler) GetRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	rec, err := h.review.GetRecord(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rec)
}

func (h *ReviewHandler) ListActions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.review.ListActions(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *ReviewHandler) Act(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.ActInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	rec, err := h.review.Act(c.Request.Context(), id, &in, uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rec)
}

func (h *ReviewHandler) Cancel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	uid := c.GetInt64("uid")
	if err := h.review.Cancel(c.Request.Context(), id, uid); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}
