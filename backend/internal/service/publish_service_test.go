package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/pkg/errcode"
)

func newPublishService(t *testing.T) (PublishService, *repo.Repositories) {
	db := newTestDB(t, &model.FullVideo{}, &model.Publish{}, &model.ReviewRecord{})
	r := newTestRepos(db)
	return &publishService{r: r, log: newNopLog()}, r
}

// ==================== Publish ====================

func TestPublishService_Publish(t *testing.T) {
	ctx := context.Background()
	s, r := newPublishService(t)

	// 预置 succeeded 的 FullVideo
	fv := &model.FullVideo{Name: "test-video", Status: "succeeded"}
	if err := r.Full.Create(ctx, fv); err != nil {
		t.Fatalf("create full video: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		p, err := s.Publish(ctx, &PublishInput{FullVideoID: fv.ID}, 100)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.FullVideoID != fv.ID {
			t.Fatalf("full_video_id=%d want %d", p.FullVideoID, fv.ID)
		}
		if p.Status != "on" {
			t.Fatalf("status=%s want on", p.Status)
		}
		if p.PublishedBy != 100 {
			t.Fatalf("published_by=%d want 100", p.PublishedBy)
		}
	})

	t.Run("with watermark config", func(t *testing.T) {
		// 创建另一个 succeeded FullVideo
		fv2 := &model.FullVideo{Name: "test-video-2", Status: "succeeded"}
		if err := r.Full.Create(ctx, fv2); err != nil {
			t.Fatalf("create full video 2: %v", err)
		}
		wm := json.RawMessage(`{"text":"hello","opacity":0.5}`)
		p, err := s.Publish(ctx, &PublishInput{FullVideoID: fv2.ID, WatermarkConfig: wm}, 101)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if string(p.WatermarkConfig) != string(wm) {
			t.Fatalf("watermark_config=%s want %s", p.WatermarkConfig, wm)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		_, err := s.Publish(ctx, nil, 100)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid full_video_id", func(t *testing.T) {
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: 0}, 100)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("negative full_video_id", func(t *testing.T) {
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: -1}, 100)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("full video not found", func(t *testing.T) {
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: 99999}, 100)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("full video status not succeeded", func(t *testing.T) {
		fvDraft := &model.FullVideo{Name: "draft-video", Status: "draft"}
		if err := r.Full.Create(ctx, fvDraft); err != nil {
			t.Fatalf("create draft video: %v", err)
		}
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: fvDraft.ID}, 100)
		if !isErr(err, errcode.ErrStateInvalid) {
			t.Fatalf("want ErrStateInvalid, got %v", err)
		}
	})

	t.Run("duplicate publish", func(t *testing.T) {
		// fv 已在 normal case 中被发布，再次发布应冲突
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: fv.ID}, 100)
		if !isErr(err, errcode.ErrConflict) {
			t.Fatalf("want ErrConflict, got %v", err)
		}
	})

	t.Run("invalid watermark config - array", func(t *testing.T) {
		fv3 := &model.FullVideo{Name: "test-video-3", Status: "succeeded"}
		if err := r.Full.Create(ctx, fv3); err != nil {
			t.Fatalf("create full video 3: %v", err)
		}
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: fv3.ID, WatermarkConfig: json.RawMessage(`[1,2]`)}, 100)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid watermark config - null", func(t *testing.T) {
		fv4 := &model.FullVideo{Name: "test-video-4", Status: "succeeded"}
		if err := r.Full.Create(ctx, fv4); err != nil {
			t.Fatalf("create full video 4: %v", err)
		}
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: fv4.ID, WatermarkConfig: json.RawMessage(`null`)}, 100)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid watermark config - literal", func(t *testing.T) {
		fv5 := &model.FullVideo{Name: "test-video-5", Status: "succeeded"}
		if err := r.Full.Create(ctx, fv5); err != nil {
			t.Fatalf("create full video 5: %v", err)
		}
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: fv5.ID, WatermarkConfig: json.RawMessage(`123`)}, 100)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid watermark config - invalid json", func(t *testing.T) {
		fv6 := &model.FullVideo{Name: "test-video-6", Status: "succeeded"}
		if err := r.Full.Create(ctx, fv6); err != nil {
			t.Fatalf("create full video 6: %v", err)
		}
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: fv6.ID, WatermarkConfig: json.RawMessage(`{invalid`)}, 100)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("review not approved", func(t *testing.T) {
		fv7 := &model.FullVideo{Name: "test-video-7", Status: "succeeded"}
		if err := r.Full.Create(ctx, fv7); err != nil {
			t.Fatalf("create full video 7: %v", err)
		}
		rec := &model.ReviewRecord{TargetType: "full_video", TargetID: fv7.ID, Status: "pending"}
		if err := r.Review.CreateRecord(ctx, rec); err != nil {
			t.Fatalf("create review record: %v", err)
		}
		_, err := s.Publish(ctx, &PublishInput{FullVideoID: fv7.ID}, 100)
		if !isErr(err, errcode.ErrStateInvalid) {
			t.Fatalf("want ErrStateInvalid, got %v", err)
		}
	})

	t.Run("review approved", func(t *testing.T) {
		fv8 := &model.FullVideo{Name: "test-video-8", Status: "succeeded"}
		if err := r.Full.Create(ctx, fv8); err != nil {
			t.Fatalf("create full video 8: %v", err)
		}
		rec := &model.ReviewRecord{TargetType: "full_video", TargetID: fv8.ID, Status: "approved"}
		if err := r.Review.CreateRecord(ctx, rec); err != nil {
			t.Fatalf("create review record: %v", err)
		}
		p, err := s.Publish(ctx, &PublishInput{FullVideoID: fv8.ID}, 100)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.Status != "on" {
			t.Fatalf("status=%s want on", p.Status)
		}
	})
}

// ==================== Unpublish ====================

func TestPublishService_Unpublish(t *testing.T) {
	ctx := context.Background()
	s, r := newPublishService(t)

	// 预置 published 记录
	fv := &model.FullVideo{Name: "test-video", Status: "succeeded"}
	if err := r.Full.Create(ctx, fv); err != nil {
		t.Fatalf("create full video: %v", err)
	}
	p, err := s.Publish(ctx, &PublishInput{FullVideoID: fv.ID}, 100)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	_ = p

	t.Run("normal", func(t *testing.T) {
		if err := s.Unpublish(ctx, fv.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// 验证状态已变为 off
		fresh, _ := r.Publish.GetByVideoID(ctx, fv.ID)
		if fresh.Status != "off" {
			t.Fatalf("status=%s want off", fresh.Status)
		}
	})

	t.Run("already off", func(t *testing.T) {
		err := s.Unpublish(ctx, fv.ID)
		if !isErr(err, errcode.ErrStateInvalid) {
			t.Fatalf("want ErrStateInvalid, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := s.Unpublish(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== Get ====================

func TestPublishService_Get(t *testing.T) {
	ctx := context.Background()
	s, r := newPublishService(t)

	// 预置 published 记录
	fv := &model.FullVideo{Name: "test-video", Status: "succeeded"}
	if err := r.Full.Create(ctx, fv); err != nil {
		t.Fatalf("create full video: %v", err)
	}
	if _, err := s.Publish(ctx, &PublishInput{FullVideoID: fv.ID}, 100); err != nil {
		t.Fatalf("publish: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		p, err := s.Get(ctx, fv.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.FullVideoID != fv.ID {
			t.Fatalf("full_video_id=%d want %d", p.FullVideoID, fv.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.Get(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== List ====================

func TestPublishService_List(t *testing.T) {
	ctx := context.Background()
	s, r := newPublishService(t)

	// 预置 3 条发布记录 (2 on, 1 off)
	for i, status := range []string{"succeeded", "succeeded", "succeeded"} {
		fv := &model.FullVideo{Name: "video-" + string(rune('a'+i)), Status: status}
		if err := r.Full.Create(ctx, fv); err != nil {
			t.Fatalf("create full video: %v", err)
		}
		if _, err := s.Publish(ctx, &PublishInput{FullVideoID: fv.ID}, int64(100+i)); err != nil {
			t.Fatalf("publish: %v", err)
		}
		if i == 2 {
			if err := s.Unpublish(ctx, fv.ID); err != nil {
				t.Fatalf("unpublish: %v", err)
			}
		}
	}

	t.Run("all", func(t *testing.T) {
		list, total, err := s.List(ctx, "", 1, 10)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 3 {
			t.Fatalf("total=%d len=%d want 3,3", total, len(list))
		}
	})

	t.Run("filter on", func(t *testing.T) {
		list, total, err := s.List(ctx, "on", 1, 10)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 2 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 2,2", total, len(list))
		}
		for _, p := range list {
			if p.Status != "on" {
				t.Fatalf("status=%s want on", p.Status)
			}
		}
	})

	t.Run("filter off", func(t *testing.T) {
		list, total, err := s.List(ctx, "off", 1, 10)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Status != "off" {
			t.Fatalf("status=%s want off", list[0].Status)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		_, _, err := s.List(ctx, "invalid", 1, 10)
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		list, total, err := s.List(ctx, "", 1, 2)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 3,2", total, len(list))
		}
	})
}

// ==================== IncPlay ====================

func TestPublishService_IncPlay(t *testing.T) {
	ctx := context.Background()
	s, r := newPublishService(t)

	// 预置 published 记录
	fv := &model.FullVideo{Name: "test-video", Status: "succeeded"}
	if err := r.Full.Create(ctx, fv); err != nil {
		t.Fatalf("create full video: %v", err)
	}
	if _, err := s.Publish(ctx, &PublishInput{FullVideoID: fv.ID}, 100); err != nil {
		t.Fatalf("publish: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.IncPlay(ctx, fv.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		p, _ := r.Publish.GetByVideoID(ctx, fv.ID)
		if p.PlayCount != 1 {
			t.Fatalf("play_count=%d want 1", p.PlayCount)
		}
		// 再次自增
		if err := s.IncPlay(ctx, fv.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		p, _ = r.Publish.GetByVideoID(ctx, fv.ID)
		if p.PlayCount != 2 {
			t.Fatalf("play_count=%d want 2", p.PlayCount)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := s.IncPlay(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("already off", func(t *testing.T) {
		fv2 := &model.FullVideo{Name: "test-video-2", Status: "succeeded"}
		if err := r.Full.Create(ctx, fv2); err != nil {
			t.Fatalf("create full video 2: %v", err)
		}
		if _, err := s.Publish(ctx, &PublishInput{FullVideoID: fv2.ID}, 100); err != nil {
			t.Fatalf("publish: %v", err)
		}
		if err := s.Unpublish(ctx, fv2.ID); err != nil {
			t.Fatalf("unpublish: %v", err)
		}
		err := s.IncPlay(ctx, fv2.ID)
		if !isErr(err, errcode.ErrStateInvalid) {
			t.Fatalf("want ErrStateInvalid, got %v", err)
		}
	})
}

// ==================== IncDownload ====================

func TestPublishService_IncDownload(t *testing.T) {
	ctx := context.Background()
	s, r := newPublishService(t)

	// 预置 published 记录
	fv := &model.FullVideo{Name: "test-video", Status: "succeeded"}
	if err := r.Full.Create(ctx, fv); err != nil {
		t.Fatalf("create full video: %v", err)
	}
	if _, err := s.Publish(ctx, &PublishInput{FullVideoID: fv.ID}, 100); err != nil {
		t.Fatalf("publish: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.IncDownload(ctx, fv.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		p, _ := r.Publish.GetByVideoID(ctx, fv.ID)
		if p.DownloadCount != 1 {
			t.Fatalf("download_count=%d want 1", p.DownloadCount)
		}
		// 再次自增
		if err := s.IncDownload(ctx, fv.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		p, _ = r.Publish.GetByVideoID(ctx, fv.ID)
		if p.DownloadCount != 2 {
			t.Fatalf("download_count=%d want 2", p.DownloadCount)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := s.IncDownload(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("already off", func(t *testing.T) {
		fv2 := &model.FullVideo{Name: "test-video-2", Status: "succeeded"}
		if err := r.Full.Create(ctx, fv2); err != nil {
			t.Fatalf("create full video 2: %v", err)
		}
		if _, err := s.Publish(ctx, &PublishInput{FullVideoID: fv2.ID}, 100); err != nil {
			t.Fatalf("publish: %v", err)
		}
		if err := s.Unpublish(ctx, fv2.ID); err != nil {
			t.Fatalf("unpublish: %v", err)
		}
		err := s.IncDownload(ctx, fv2.ID)
		if !isErr(err, errcode.ErrStateInvalid) {
			t.Fatalf("want ErrStateInvalid, got %v", err)
		}
	})
}

// ==================== UpdateWatermark ====================

func TestPublishService_UpdateWatermark(t *testing.T) {
	ctx := context.Background()
	s, r := newPublishService(t)

	// 预置 published 记录
	fv := &model.FullVideo{Name: "test-video", Status: "succeeded"}
	if err := r.Full.Create(ctx, fv); err != nil {
		t.Fatalf("create full video: %v", err)
	}
	if _, err := s.Publish(ctx, &PublishInput{FullVideoID: fv.ID}, 100); err != nil {
		t.Fatalf("publish: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		wm := json.RawMessage(`{"text":"updated","x":10,"y":20}`)
		p, err := s.UpdateWatermark(ctx, fv.ID, wm)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if string(p.WatermarkConfig) != string(wm) {
			t.Fatalf("watermark_config=%s want %s", p.WatermarkConfig, wm)
		}
	})

	t.Run("empty watermark", func(t *testing.T) {
		_, err := s.UpdateWatermark(ctx, fv.ID, json.RawMessage{})
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("null watermark", func(t *testing.T) {
		_, err := s.UpdateWatermark(ctx, fv.ID, json.RawMessage(`null`))
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("array watermark", func(t *testing.T) {
		_, err := s.UpdateWatermark(ctx, fv.ID, json.RawMessage(`[1,2]`))
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("literal watermark", func(t *testing.T) {
		_, err := s.UpdateWatermark(ctx, fv.ID, json.RawMessage(`123`))
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("invalid json watermark", func(t *testing.T) {
		_, err := s.UpdateWatermark(ctx, fv.ID, json.RawMessage(`{invalid`))
		if !isErr(err, errcode.ErrParam) {
			t.Fatalf("want ErrParam, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.UpdateWatermark(ctx, 99999, json.RawMessage(`{"text":"ok"}`))
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}
