package handler

import (
	"strconv"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type BillingHandler struct {
	billing service.BillingService
	log     *zap.Logger
}

func (h *BillingHandler) ListQuotas(c *gin.Context) {
	scopeType := c.Query("scope_type")
	scopeID, err := strconv.ParseInt(c.Query("scope_id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.billing.ListQuotas(c.Request.Context(), scopeType, scopeID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *BillingHandler) GetQuota(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	q, err := h.billing.GetQuota(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, q)
}

func (h *BillingHandler) CreateQuota(c *gin.Context) {
	var in service.CreateQuotaInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, err)
		return
	}
	q, err := h.billing.CreateQuota(c.Request.Context(), &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, q)
}

func (h *BillingHandler) UpdateQuota(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	var in service.UpdateQuotaInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, err)
		return
	}
	q, err := h.billing.UpdateQuota(c.Request.Context(), id, &in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, q)
}

func (h *BillingHandler) DeleteQuota(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	if err := h.billing.DeleteQuota(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *BillingHandler) ListDaily(c *gin.Context) {
	const layout = "2006-01-02"
	now := time.Now()
	from := now.AddDate(0, 0, -30)
	to := now
	if s := c.Query("from"); s != "" {
		if t, err := time.Parse(layout, s); err == nil {
			from = t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := time.Parse(layout, s); err == nil {
			to = t
		}
	}
	userID, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	deptID, err := strconv.ParseInt(c.Query("dept_id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	modelID, err := strconv.ParseInt(c.Query("model_id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
		return
	}
	list, err := h.billing.ListDaily(c.Request.Context(), from, to, userID, deptID, modelID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}
