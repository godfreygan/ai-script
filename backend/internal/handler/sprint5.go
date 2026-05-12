// Sprint 5 HTTP 入口:Review / Publish / Billing / Audit
//
// 设计要点:
//   - 所有 handler 都返回 response.OK / response.Fail,与 sprint3/4 一致
//   - 入参绑定走 service.* 已有 Input 结构,不再造轮子
//   - 列表统一返回 response.Page,单条返回原 model
//   - publish 类操作以 full_video_id 为 :id(因为 publishes 表对每个 full_video 唯一)
package handler

import (
	"encoding/json"
	"strconv"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/service"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ===================== ReviewHandler =====================

type ReviewHandler struct {
	review *service.ReviewService
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	f, err := h.review.GetFlow(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, f)
}

func (h *ReviewHandler) ListNodes(c *gin.Context) {
	flowID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := h.review.ListRecords(c.Request.Context(), status, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: page, PageSize: pageSize, Total: total})
}

func (h *ReviewHandler) GetRecord(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rec, err := h.review.GetRecord(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rec)
}

func (h *ReviewHandler) ListActions(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.review.ListActions(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *ReviewHandler) Act(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid := c.GetInt64("uid")
	if err := h.review.Cancel(c.Request.Context(), id, uid); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// ===================== PublishHandler =====================

type PublishHandler struct {
	publish *service.PublishService
	log     *zap.Logger
}

// Publish POST /publishes  body: {full_video_id, watermark_config}
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

// Unpublish POST /publishes/:id/unpublish  :id = full_video_id
func (h *PublishHandler) Unpublish(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.publish.Unpublish(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// Get GET /publishes/:id  :id = full_video_id
func (h *PublishHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	p, err := h.publish.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}

func (h *PublishHandler) List(c *gin.Context) {
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := h.publish.List(c.Request.Context(), status, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: page, PageSize: pageSize, Total: total})
}

func (h *PublishHandler) IncPlay(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.publish.IncPlay(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

func (h *PublishHandler) IncDownload(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.publish.IncDownload(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// UpdateWatermark PUT /publishes/:id/watermark  body: {"watermark_config": {...}}
func (h *PublishHandler) UpdateWatermark(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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

// ===================== BillingHandler =====================

type BillingHandler struct {
	billing *service.BillingService
	log     *zap.Logger
}

// ListQuotas GET /billing/quotas?scope_type=user|dept&scope_id=N
func (h *BillingHandler) ListQuotas(c *gin.Context) {
	scopeType := c.Query("scope_type")
	scopeID, _ := strconv.ParseInt(c.Query("scope_id"), 10, 64)
	list, err := h.billing.ListQuotas(c.Request.Context(), scopeType, scopeID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *BillingHandler) GetQuota(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
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
		response.Fail(c, errcode.ErrParam.Wrap(err))
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in service.UpdateQuotaInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.ErrParam.Wrap(err))
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
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.billing.DeleteQuota(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// ListDaily GET /billing/daily?from=2026-01-01&to=2026-01-31&user_id=&dept_id=&model_id=
// 时间格式: YYYY-MM-DD,from/to 不传时分别默认为 30 天前 / 今天
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
	userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	deptID, _ := strconv.ParseInt(c.Query("dept_id"), 10, 64)
	modelID, _ := strconv.ParseInt(c.Query("model_id"), 10, 64)
	list, err := h.billing.ListDaily(c.Request.Context(), from, to, userID, deptID, modelID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// ===================== AuditHandler =====================

type AuditHandler struct {
	audit *service.AuditService
	log   *zap.Logger
}

func (h *AuditHandler) List(c *gin.Context) {
	q := &repo.ListAuditQuery{}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	q.UserID, _ = strconv.ParseInt(c.Query("user_id"), 10, 64)
	q.ResourceType = c.Query("resource_type")
	q.Action = c.Query("action")
	list, total, err := h.audit.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.Page{List: list, Page: q.Page, PageSize: q.PageSize, Total: total})
}
