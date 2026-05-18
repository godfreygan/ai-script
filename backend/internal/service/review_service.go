package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ReviewService 审核流程业务逻辑
type reviewService struct {
	r   *repo.Repositories
	log *zap.Logger
}

// NewReviewService 构造审核服务
func NewReviewService(r *repo.Repositories, log *zap.Logger) *reviewService {
	return &reviewService{r: r, log: log}
}

// SubmitInput 提交审核
type SubmitInput struct {
	TargetType string `json:"target_type" binding:"required,oneof=full_video"` // 默认 "full_video"
	TargetID   int64  `json:"target_id" binding:"required,gte=1"`
	FlowID     int64  `json:"flow_id" binding:"omitempty,gte=1"` // 0 = 使用 default flow
	Note       string `json:"note" binding:"omitempty,max=500"`
}

// ActInput 审批/驳回/转交
type ActInput struct {
	Action  string `json:"action" binding:"required,oneof=approve reject skip"` // approve / reject / skip
	Comment string `json:"comment" binding:"omitempty,max=500"`
}

// ListFlows 列出所有启用的审核流
func (s *reviewService) ListFlows(ctx context.Context) ([]model.ReviewFlow, error) {
	list, err := s.r.Review.ListFlows(ctx)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return list, nil
}

// GetFlow 获取审核流详情
func (s *reviewService) GetFlow(ctx context.Context, id int64) (*model.ReviewFlow, error) {
	f, err := s.r.Review.GetFlow(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return f, nil
}

// ListNodes 列出审核流的节点
func (s *reviewService) ListNodes(ctx context.Context, flowID int64) ([]model.ReviewNode, error) {
	list, err := s.r.Review.ListNodes(ctx, flowID)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return list, nil
}

// Submit 提交审核
func (s *reviewService) Submit(ctx context.Context, in *SubmitInput, uid int64) (*model.ReviewRecord, error) {
	if in == nil {
		return nil, errcode.ErrParam.WithMsg("缺少提交参数")
	}
	targetType := in.TargetType
	if targetType == "" {
		targetType = "full_video"
	}
	if in.TargetID <= 0 {
		return nil, errcode.ErrParam.WithMsg("target_id 不能为空")
	}

	// 校验 target 是否存在(仅对 full_video)
	if targetType == "full_video" {
		if _, err := s.r.Full.Get(ctx, in.TargetID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errcode.ErrNotFound.WithMsg("待审核视频不存在")
			}
			return nil, errcode.ErrInternal.Wrap(err)
		}
	}

	// 是否已有 active record
	active, err := s.r.Review.GetActiveRecord(ctx, targetType, in.TargetID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if active != nil {
		return nil, errcode.ErrConflict.WithMsg("已有进行中的审核")
	}

	// 解析 flow
	flowID := in.FlowID
	if flowID == 0 {
		def, err := s.r.Review.DefaultFlow(ctx, targetType)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errcode.ErrNotFound.WithMsg("未配置默认审核流")
			}
			return nil, errcode.ErrInternal.Wrap(err)
		}
		flowID = def.ID
	} else {
		// 校验指定的 flow 存在
		if _, err := s.r.Review.GetFlow(ctx, flowID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errcode.ErrNotFound.WithMsg("审核流不存在")
			}
			return nil, errcode.ErrInternal.Wrap(err)
		}
	}

	// 校验 flow 有节点
	nodes, err := s.r.Review.ListNodes(ctx, flowID)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if len(nodes) == 0 {
		return nil, errcode.ErrParam.WithMsg("审核流未配置节点")
	}

	// 取第一节点的 step_no 作为初始 current_step
	// (ListNodes 已按 step_no 升序;不假设最小 step_no = 1)
	firstStep := nodes[0].StepNo
	if firstStep <= 0 {
		firstStep = 1
	}
	rec := &model.ReviewRecord{
		TargetType:  targetType,
		TargetID:    in.TargetID,
		FlowID:      flowID,
		CurrentStep: firstStep,
		Status:      "pending",
		SubmittedBy: uid,
	}
	if err := s.r.Review.CreateRecord(ctx, rec); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return rec, nil
}

// ListRecords 列出审核记录
func (s *reviewService) ListRecords(ctx context.Context, status string, page, size int) ([]model.ReviewRecord, int64, error) {
	list, total, err := s.r.Review.ListRecords(ctx, status, page, size)
	if err != nil {
		return nil, 0, errcode.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

// GetRecord 获取审核记录详情
func (s *reviewService) GetRecord(ctx context.Context, id int64) (*model.ReviewRecord, error) {
	rec, err := s.r.Review.GetRecord(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return rec, nil
}

// ListActions 列出某条审核记录的所有节点动作
func (s *reviewService) ListActions(ctx context.Context, recordID int64) ([]model.ReviewNodeRecord, error) {
	list, err := s.r.Review.ListNodeRecords(ctx, recordID)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	return list, nil
}

// Act 执行审批/驳回/跳过
func (s *reviewService) Act(ctx context.Context, recordID int64, in *ActInput, uid int64) (*model.ReviewRecord, error) {
	if in == nil {
		return nil, errcode.ErrParam.WithMsg("缺少操作参数")
	}
	switch in.Action {
	case "approve", "reject", "skip":
	default:
		return nil, errcode.ErrParam.WithMsg("action 必须是 approve/reject/skip")
	}

	rec, err := s.GetRecord(ctx, recordID)
	if err != nil {
		return nil, err
	}
	if rec.Status != "pending" {
		return nil, errcode.ErrConflict.WithMsg("审核已结束")
	}

	nodes, err := s.r.Review.ListNodes(ctx, rec.FlowID)
	if err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}
	if len(nodes) == 0 {
		return nil, errcode.ErrParam.WithMsg("审核流未配置节点")
	}

	// 找当前 step 对应的 node
	var current *model.ReviewNode
	maxStep := 0
	for i := range nodes {
		if nodes[i].StepNo > maxStep {
			maxStep = nodes[i].StepNo
		}
		if nodes[i].StepNo == rec.CurrentStep {
			current = &nodes[i]
		}
	}
	if current == nil {
		return nil, errcode.ErrParam.WithMsg("当前审核节点不存在")
	}

	// 审批人校验(MVP):approver_type=user 时 ApproverValue 必须等于 uid
	if current.ApproverType == "user" {
		if current.ApproverValue != strconv.FormatInt(uid, 10) {
			return nil, errcode.ErrForbidden.WithMsg("当前节点无权审批")
		}
	}
	// role / dept 暂不校验,留待 Casbin 接入

	// skip 需要 AllowTimeoutPass=1
	if in.Action == "skip" && current.AllowTimeoutPass != 1 {
		return nil, errcode.ErrForbidden.WithMsg("当前节点不允许跳过")
	}

	// 写节点动作
	nr := &model.ReviewNodeRecord{
		ReviewRecordID: recordID,
		StepNo:         rec.CurrentStep,
		ApproverID:     uid,
		Action:         in.Action,
		Comment:        in.Comment,
	}
	if err := s.r.Review.AddNodeRecord(ctx, nr); err != nil {
		return nil, errcode.ErrInternal.Wrap(err)
	}

	now := time.Now()
	fields := map[string]any{}
	switch in.Action {
	case "approve", "skip":
		// 找下一个 step
		nextStep := 0
		for _, n := range nodes {
			if n.StepNo > rec.CurrentStep {
				if nextStep == 0 || n.StepNo < nextStep {
					nextStep = n.StepNo
				}
			}
		}
		if nextStep > 0 {
			fields["current_step"] = nextStep
		} else {
			// 已是最后节点
			if in.Action == "approve" {
				fields["status"] = "approved"
				fields["finished_at"] = now
				fields["current_step"] = maxStep
			}
			// skip 在最后节点保持 pending,不写无意义更新(避免触发 CAS 失败)
		}
	case "reject":
		fields["status"] = "rejected"
		fields["finished_at"] = now
	}

	if len(fields) > 0 {
		// 修复 P0 #6 — 真 CAS:让 MySQL 通过 WHERE id=? AND status=? AND current_step=?
		// 单条 SQL 完成读+写,RowsAffected=0 即视为并发冲突。
		// 原假 CAS(GetRecord→比较→UpdateRecord)在两步之间仍有窗口,双 reviewer
		// 同时 approve 会双写;真 CAS 由 DB 行锁兜底,彻底消除该窗口。
		ok, cerr := s.r.Review.UpdateRecordCAS(ctx, recordID, "pending", rec.CurrentStep, fields)
		if cerr != nil {
			return nil, errcode.ErrInternal.Wrap(cerr)
		}
		if !ok {
			return nil, errcode.ErrConflict.WithMsg("审核状态已变更,请刷新后重试")
		}
	}

	return s.GetRecord(ctx, recordID)
}

// Cancel 撤回审核(仅提交人可撤)
func (s *reviewService) Cancel(ctx context.Context, recordID int64, uid int64) error {
	rec, err := s.GetRecord(ctx, recordID)
	if err != nil {
		return err
	}
	if rec.SubmittedBy != uid {
		return errcode.ErrForbidden.WithMsg("只有提交人可撤回审核")
	}
	if rec.Status != "pending" {
		return errcode.ErrConflict.WithMsg("审核已结束,无法撤回")
	}
	now := time.Now()
	fields := map[string]any{
		"status":      "cancelled",
		"finished_at": now,
	}
	// 修复 P0 #6 — Cancel 与并发 Act 之间也存在 race:Act 可能已抢先
	// approve/reject(status 改变),旧 rec 仍为 pending,撤回会覆盖已结审。
	// 用 CAS 让 DB 兜底:WHERE status=? AND current_step=?,RowsAffected=0 即冲突。
	ok, cerr := s.r.Review.UpdateRecordCAS(ctx, recordID, "pending", rec.CurrentStep, fields)
	if cerr != nil {
		return errcode.ErrInternal.Wrap(cerr)
	}
	if !ok {
		return errcode.ErrConflict.WithMsg("审核状态已变更,无法撤回")
	}
	return nil
}
