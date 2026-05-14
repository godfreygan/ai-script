package service

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
)

// newReviewService 用 in-memory sqlite 启 ReviewService。
// 注意:Submit 需要 FullVideo 表(校验 target),所以也迁过来。
func newReviewService(t *testing.T) (ReviewService, *repo.Repositories) {
	db := newTestDB(t,
		&model.ReviewFlow{}, &model.ReviewNode{},
		&model.ReviewRecord{}, &model.ReviewNodeRecord{},
		&model.FullVideo{},
	)
	r := newTestRepos(db)
	return NewReviewService(r, zap.NewNop()), r
}

// seedFlow 建一个有 N 个 step 的审核流(每个 step 一个 user 审批节点)。
func seedFlow(t *testing.T, r *repo.Repositories, approvers []int64, allowSkip []bool) *model.ReviewFlow {
	t.Helper()
	flow := &model.ReviewFlow{Name: "test", TargetType: "full_video", Enabled: 1, IsDefault: 1}
	if err := r.DB.Create(flow).Error; err != nil {
		t.Fatalf("create flow: %v", err)
	}
	for i, uid := range approvers {
		node := &model.ReviewNode{
			FlowID:        flow.ID,
			StepNo:        i + 1,
			Name:          "step" + strconv.Itoa(i+1),
			ApproverType:  "user",
			ApproverValue: strconv.FormatInt(uid, 10),
		}
		if allowSkip[i] {
			node.AllowTimeoutPass = 1
		}
		if err := r.DB.Create(node).Error; err != nil {
			t.Fatalf("create node %d: %v", i, err)
		}
	}
	return flow
}

// seedTarget 建一个 FullVideo target,返回 id。
func seedTarget(t *testing.T, r *repo.Repositories) int64 {
	t.Helper()
	v := &model.FullVideo{Name: "v1", Status: "draft"}
	if err := r.DB.Create(v).Error; err != nil {
		t.Fatalf("create target: %v", err)
	}
	return v.ID
}

// seedRecord 建一条 pending 审核记录(关联到 flow,起始 step=1)。
func seedRecord(t *testing.T, r *repo.Repositories, flowID, targetID, submitter int64) *model.ReviewRecord {
	t.Helper()
	rec := &model.ReviewRecord{
		TargetType: "full_video", TargetID: targetID,
		FlowID: flowID, CurrentStep: 1,
		Status: "pending", SubmittedBy: submitter,
	}
	if err := r.Review.CreateRecord(context.Background(), rec); err != nil {
		t.Fatalf("create record: %v", err)
	}
	return rec
}

func TestReviewService_Act(t *testing.T) {
	ctx := context.Background()

	type setupFn func(t *testing.T, r *repo.Repositories) (recordID int64, callerUID int64)

	cases := []struct {
		name       string
		setup      setupFn
		in         *ActInput
		wantErr    *errcode.Error
		wantStatus string
		wantStep   int
	}{
		{
			name: "approve advances to next step",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1001, 1002}, []bool{false, false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 9000)
				return rec.ID, 1001
			},
			in:         &ActInput{Action: "approve"},
			wantStatus: "pending",
			wantStep:   2,
		},
		{
			name: "approve at final step finalizes",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1001}, []bool{false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 9000)
				return rec.ID, 1001
			},
			in:         &ActInput{Action: "approve"},
			wantStatus: "approved",
			wantStep:   1,
		},
		{
			name: "reject short-circuits",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1001, 1002, 1003}, []bool{false, false, false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 9000)
				return rec.ID, 1001
			},
			in:         &ActInput{Action: "reject", Comment: "no"},
			wantStatus: "rejected",
			wantStep:   1,
		},
		{
			name: "skip forbidden when AllowTimeoutPass=0",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1001, 1002}, []bool{false, false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 9000)
				return rec.ID, 1001
			},
			in:      &ActInput{Action: "skip"},
			wantErr: errcode.ErrForbidden,
		},
		{
			name: "skip allowed when AllowTimeoutPass=1",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1001, 1002}, []bool{true, false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 9000)
				return rec.ID, 1001
			},
			in:         &ActInput{Action: "skip"},
			wantStatus: "pending",
			wantStep:   2,
		},
		{
			name: "approver mismatch forbidden",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1001}, []bool{false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 9000)
				return rec.ID, 5555 // 错的人
			},
			in:      &ActInput{Action: "approve"},
			wantErr: errcode.ErrForbidden,
		},
		{
			name: "invalid action returns param err",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1001}, []bool{false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 9000)
				return rec.ID, 1001
			},
			in:      &ActInput{Action: "explode"},
			wantErr: errcode.ErrParam,
		},
		{
			name: "act on finished record returns conflict",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1001}, []bool{false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 9000)
				// 把 record 标记为 approved
				if err := r.Review.UpdateRecord(context.Background(), rec.ID, map[string]any{"status": "approved"}); err != nil {
					t.Fatalf("update: %v", err)
				}
				return rec.ID, 1001
			},
			in:      &ActInput{Action: "approve"},
			wantErr: errcode.ErrConflict,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, r := newReviewService(t)
			recID, uid := c.setup(t, r)
			got, err := s.Act(ctx, recID, c.in, uid)
			if c.wantErr != nil {
				if err == nil {
					t.Fatalf("expect err %v, got nil", c.wantErr)
				}
				var ec *errcode.Error
				if !errors.As(err, &ec) || ec.Code != c.wantErr.Code {
					t.Fatalf("want errcode %v, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.Status != c.wantStatus {
				t.Errorf("status=%s want %s", got.Status, c.wantStatus)
			}
			if got.CurrentStep != c.wantStep {
				t.Errorf("step=%d want %d", got.CurrentStep, c.wantStep)
			}
		})
	}
}

func TestReviewService_Cancel(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		setup   func(t *testing.T, r *repo.Repositories) (recordID int64, callerUID int64)
		wantErr *errcode.Error
	}{
		{
			name: "submitter can cancel pending",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1}, []bool{false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 777)
				return rec.ID, 777
			},
		},
		{
			name: "non-submitter forbidden",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1}, []bool{false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 777)
				return rec.ID, 999
			},
			wantErr: errcode.ErrForbidden,
		},
		{
			name: "cancel finished returns conflict",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				f := seedFlow(t, r, []int64{1}, []bool{false})
				tgt := seedTarget(t, r)
				rec := seedRecord(t, r, f.ID, tgt, 777)
				if err := r.Review.UpdateRecord(context.Background(), rec.ID, map[string]any{"status": "rejected"}); err != nil {
					t.Fatalf("update: %v", err)
				}
				return rec.ID, 777
			},
			wantErr: errcode.ErrConflict,
		},
		{
			name: "cancel missing record",
			setup: func(t *testing.T, r *repo.Repositories) (int64, int64) {
				return 99999, 1
			},
			wantErr: errcode.ErrNotFound,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, r := newReviewService(t)
			recID, uid := c.setup(t, r)
			err := s.Cancel(ctx, recID, uid)
			if c.wantErr != nil {
				if err == nil {
					t.Fatalf("expect err %v, got nil", c.wantErr)
				}
				var ec *errcode.Error
				if !errors.As(err, &ec) || ec.Code != c.wantErr.Code {
					t.Fatalf("want %v, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			// 确认状态变 cancelled
			rec, _ := r.Review.GetRecord(ctx, recID)
			if rec.Status != "cancelled" {
				t.Errorf("status=%s want cancelled", rec.Status)
			}
		})
	}
}
