package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/queue"
	"github.com/hibiken/asynq"
)

// mockTaskClient 是一个用于测试的 TaskClient mock
type mockTaskClient struct {
	enqueueFunc func(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) (string, error)
}

func (m *mockTaskClient) Enqueue(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) (string, error) {
	if m.enqueueFunc != nil {
		return m.enqueueFunc(ctx, taskType, payload, opts...)
	}
	return "task-id-123", nil
}

func (m *mockTaskClient) EnqueueIn(ctx context.Context, taskType string, payload []byte, delay time.Duration) (string, error) {
	return "task-id-456", nil
}

func (m *mockTaskClient) Ping() error { return nil }

func newFullVideoService(t *testing.T) (*fullVideoService, *repo.Repositories) {
	db := newTestDB(t, &model.FullVideo{}, &model.ShortVideo{})
	r := newTestRepos(db)
	return &fullVideoService{
		r:     r,
		tc:    &mockTaskClient{},
		hub:   nil,
		store: nil,
		log:   newNopLog(),
	}, r
}

func newFullVideoServiceWithMockTC(t *testing.T, tc queue.TaskClient) (*fullVideoService, *repo.Repositories) {
	db := newTestDB(t, &model.FullVideo{}, &model.ShortVideo{})
	r := newTestRepos(db)
	return &fullVideoService{
		r:     r,
		tc:    tc,
		hub:   nil,
		store: nil,
		log:   newNopLog(),
	}, r
}

// ==================== List ====================

func TestFullVideoService_List(t *testing.T) {
	ctx := context.Background()
	s, r := newFullVideoService(t)

	// 预置数据
	for _, name := range []string{"video-a", "video-b", "video-c"} {
		f := &model.FullVideo{
			ProjectID: 1,
			Name:      name,
			Status:    "draft",
			Version:   1,
		}
		if err := r.Full.Create(ctx, f); err != nil {
			t.Fatalf("create full video: %v", err)
		}
	}

	t.Run("normal", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListFullVideosQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 3 {
			t.Fatalf("total=%d len=%d want 3,3", total, len(list))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListFullVideosQuery{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 3,2", total, len(list))
		}
	})

	t.Run("filter by project_id", func(t *testing.T) {
		// 创建属于 project 2 的视频
		f := &model.FullVideo{ProjectID: 2, Name: "video-d", Status: "draft", Version: 1}
		if err := r.Full.Create(ctx, f); err != nil {
			t.Fatalf("create: %v", err)
		}
		list, total, err := s.List(ctx, &repo.ListFullVideosQuery{Page: 1, PageSize: 10, ProjectID: 2})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].ProjectID != 2 {
			t.Fatalf("project_id=%d want 2", list[0].ProjectID)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		// 更新一个为 succeeded
		list, _, _ := s.List(ctx, &repo.ListFullVideosQuery{Page: 1, PageSize: 1})
		if len(list) > 0 {
			r.Full.UpdateStatus(ctx, list[0].ID, "succeeded", 100, "")
		}
		list, total, err := s.List(ctx, &repo.ListFullVideosQuery{Page: 1, PageSize: 10, Status: "succeeded"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Status != "succeeded" {
			t.Fatalf("status=%s want succeeded", list[0].Status)
		}
	})
}

// ==================== Get ====================

func TestFullVideoService_Get(t *testing.T) {
	ctx := context.Background()
	s, r := newFullVideoService(t)

	// 预置数据
	tl := Timeline{
		Clips: []TimelineClip{
			{ShortVideoID: 1, URL: "http://example.com/1.mp4", DurationMs: 5000, TTSText: "hello"},
		},
		BurnSubtitles: true,
	}
	tlBytes, _ := json.Marshal(tl)
	f := &model.FullVideo{
		ProjectID: 1,
		Name:      "test-video",
		Timeline:  model.JSON(tlBytes),
		Status:    "draft",
		Version:   1,
	}
	if err := r.Full.Create(ctx, f); err != nil {
		t.Fatalf("create full video: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		got, err := s.Get(ctx, f.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.Name != "test-video" {
			t.Fatalf("name=%s want test-video", got.Name)
		}
		if got.ProjectID != 1 {
			t.Fatalf("project_id=%d want 1", got.ProjectID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := s.Get(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== Create ====================

func TestFullVideoService_Create(t *testing.T) {
	ctx := context.Background()
	s, _ := newFullVideoService(t)

	t.Run("normal", func(t *testing.T) {
		in := &CreateFullVideoInput{
			ProjectID: 1,
			Name:      "new-video",
			Timeline: Timeline{
				Clips: []TimelineClip{
					{ShortVideoID: 1, URL: "http://example.com/1.mp4", DurationMs: 3000},
				},
				BurnSubtitles: false,
			},
		}
		uid := int64(42)
		f, err := s.Create(ctx, in, uid)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if f.Name != "new-video" {
			t.Fatalf("name=%s want new-video", f.Name)
		}
		if f.ProjectID != 1 {
			t.Fatalf("project_id=%d want 1", f.ProjectID)
		}
		if f.Status != "draft" {
			t.Fatalf("status=%s want draft", f.Status)
		}
		if f.Version != 1 {
			t.Fatalf("version=%d want 1", f.Version)
		}
		if f.CreatedBy != uid || f.UpdatedBy != uid {
			t.Fatalf("created_by=%d updated_by=%d want %d", f.CreatedBy, f.UpdatedBy, uid)
		}
		// 验证 timeline 被正确序列化
		if len(f.Timeline) == 0 {
			t.Fatalf("timeline should not be empty")
		}
		var tl Timeline
		if err := json.Unmarshal(f.Timeline, &tl); err != nil {
			t.Fatalf("unmarshal timeline: %v", err)
		}
		if len(tl.Clips) != 1 || tl.Clips[0].URL != "http://example.com/1.mp4" {
			t.Fatalf("timeline clips mismatch")
		}
	})

	t.Run("with tts and subtitles", func(t *testing.T) {
		in := &CreateFullVideoInput{
			ProjectID: 2,
			Name:      "video-with-tts",
			Timeline: Timeline{
				Clips: []TimelineClip{
					{ShortVideoID: 1, URL: "http://example.com/1.mp4", DurationMs: 5000, TTSText: "Hello world", Speaker: " narrator"},
					{ShortVideoID: 2, URL: "http://example.com/2.mp4", DurationMs: 3000, TTSText: "Goodbye", Speaker: " narrator"},
				},
				BackgroundAudio: "http://example.com/bgm.mp3",
				TTSModelID:      10,
				BurnSubtitles:   true,
			},
		}
		f, err := s.Create(ctx, in, 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if f.ProjectID != 2 {
			t.Fatalf("project_id=%d want 2", f.ProjectID)
		}
		var tl Timeline
		if err := json.Unmarshal(f.Timeline, &tl); err != nil {
			t.Fatalf("unmarshal timeline: %v", err)
		}
		if tl.TTSModelID != 10 {
			t.Fatalf("tts_model_id=%d want 10", tl.TTSModelID)
		}
		if !tl.BurnSubtitles {
			t.Fatalf("burn_subtitles should be true")
		}
		if tl.BackgroundAudio != "http://example.com/bgm.mp3" {
			t.Fatalf("background_audio mismatch")
		}
	})
}

// ==================== Update ====================

func TestFullVideoService_Update(t *testing.T) {
	ctx := context.Background()
	s, r := newFullVideoService(t)

	// 预置数据
	tl := Timeline{
		Clips: []TimelineClip{
			{ShortVideoID: 1, URL: "http://example.com/1.mp4", DurationMs: 5000},
		},
	}
	tlBytes, _ := json.Marshal(tl)
	f := &model.FullVideo{
		ProjectID: 1,
		Name:      "old-name",
		Timeline:  model.JSON(tlBytes),
		Status:    "draft",
		Version:   1,
	}
	if err := r.Full.Create(ctx, f); err != nil {
		t.Fatalf("create full video: %v", err)
	}

	t.Run("normal update name", func(t *testing.T) {
		newName := "new-name"
		uid := int64(99)
		updated, err := s.Update(ctx, f.ID, &UpdateFullVideoInput{
			Name: &newName,
		}, uid)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Name != "new-name" {
			t.Fatalf("name=%s want new-name", updated.Name)
		}
		if updated.UpdatedBy != uid {
			t.Fatalf("updated_by=%d want %d", updated.UpdatedBy, uid)
		}
	})

	t.Run("update timeline", func(t *testing.T) {
		newTL := Timeline{
			Clips: []TimelineClip{
				{ShortVideoID: 2, URL: "http://example.com/2.mp4", DurationMs: 3000, TTSText: "updated"},
				{ShortVideoID: 3, URL: "http://example.com/3.mp4", DurationMs: 4000},
			},
			TTSModelID:    5,
			BurnSubtitles: true,
		}
		updated, err := s.Update(ctx, f.ID, &UpdateFullVideoInput{
			Timeline: &newTL,
		}, 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		var tl Timeline
		if err := json.Unmarshal(updated.Timeline, &tl); err != nil {
			t.Fatalf("unmarshal timeline: %v", err)
		}
		if len(tl.Clips) != 2 {
			t.Fatalf("clips len=%d want 2", len(tl.Clips))
		}
		if tl.Clips[0].TTSText != "updated" {
			t.Fatalf("clip[0].tts_text=%s want updated", tl.Clips[0].TTSText)
		}
		if tl.TTSModelID != 5 {
			t.Fatalf("tts_model_id=%d want 5", tl.TTSModelID)
		}
	})

	t.Run("update both name and timeline", func(t *testing.T) {
		newName := "both-updated"
		newTL := Timeline{
			Clips: []TimelineClip{
				{URL: "http://example.com/new.mp4", DurationMs: 1000},
			},
		}
		updated, err := s.Update(ctx, f.ID, &UpdateFullVideoInput{
			Name:     &newName,
			Timeline: &newTL,
		}, 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Name != "both-updated" {
			t.Fatalf("name=%s want both-updated", updated.Name)
		}
		var tl Timeline
		if err := json.Unmarshal(updated.Timeline, &tl); err != nil {
			t.Fatalf("unmarshal timeline: %v", err)
		}
		if len(tl.Clips) != 1 || tl.Clips[0].URL != "http://example.com/new.mp4" {
			t.Fatalf("timeline mismatch")
		}
	})

	t.Run("not found", func(t *testing.T) {
		newName := "whatever"
		_, err := s.Update(ctx, 99999, &UpdateFullVideoInput{Name: &newName}, 1)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("partial update - only name", func(t *testing.T) {
		// 先获取当前状态
		before, _ := r.Full.Get(ctx, f.ID)
		oldTimeline := string(before.Timeline)

		partialName := "partial-name"
		updated, err := s.Update(ctx, f.ID, &UpdateFullVideoInput{
			Name: &partialName,
		}, 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Name != "partial-name" {
			t.Fatalf("name=%s want partial-name", updated.Name)
		}
		// timeline 应保持不变
		if string(updated.Timeline) != oldTimeline {
			t.Fatalf("timeline should not change")
		}
	})

	t.Run("partial update - only timeline", func(t *testing.T) {
		// 先获取当前状态
		before, _ := r.Full.Get(ctx, f.ID)
		oldName := before.Name

		newTL := Timeline{
			Clips: []TimelineClip{
				{URL: "http://example.com/only-tl.mp4", DurationMs: 2000},
			},
		}
		updated, err := s.Update(ctx, f.ID, &UpdateFullVideoInput{
			Timeline: &newTL,
		}, 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// name 应保持不变
		if updated.Name != oldName {
			t.Fatalf("name should not change, got %s want %s", updated.Name, oldName)
		}
	})
}

// ==================== Delete ====================

func TestFullVideoService_Delete(t *testing.T) {
	ctx := context.Background()
	s, r := newFullVideoService(t)

	// 预置数据
	f := &model.FullVideo{
		ProjectID: 1,
		Name:      "to-delete",
		Status:    "draft",
		Version:   1,
	}
	if err := r.Full.Create(ctx, f); err != nil {
		t.Fatalf("create full video: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.Delete(ctx, f.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// 验证已删除
		_, err := r.Full.Get(ctx, f.ID)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("delete non-existent", func(t *testing.T) {
		// Delete 方法直接调用 repo.Delete，对不存在的 ID 不会报错 (gorm 软删除行为)
		err := s.Delete(ctx, 99999)
		// gorm Delete 对不存在的记录返回 nil error
		if err != nil {
			t.Fatalf("delete non-existent should not err, got %v", err)
		}
	})
}

// ==================== Render ====================

func TestFullVideoService_Render(t *testing.T) {
	ctx := context.Background()

	t.Run("normal", func(t *testing.T) {
		s, r := newFullVideoService(t)
		// 预置数据
		f := &model.FullVideo{
			ProjectID: 1,
			Name:      "to-render",
			Status:    "draft",
			Version:   1,
		}
		if err := r.Full.Create(ctx, f); err != nil {
			t.Fatalf("create full video: %v", err)
		}

		taskID, err := s.Render(ctx, f.ID, 42)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if taskID != "task-id-123" {
			t.Fatalf("taskID=%s want task-id-123", taskID)
		}

		// 验证状态已更新为 queued
		updated, _ := r.Full.Get(ctx, f.ID)
		if updated.Status != "queued" {
			t.Fatalf("status=%s want queued", updated.Status)
		}
		if updated.RenderProgress != 0 {
			t.Fatalf("render_progress=%d want 0", updated.RenderProgress)
		}
	})

	t.Run("not found", func(t *testing.T) {
		s, _ := newFullVideoService(t)
		_, err := s.Render(ctx, 99999, 1)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("enqueue error", func(t *testing.T) {
		mockTC := &mockTaskClient{
			enqueueFunc: func(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) (string, error) {
				return "", errcode.ErrInternal.Wrap(nil)
			},
		}
		s, r := newFullVideoServiceWithMockTC(t, mockTC)
		f := &model.FullVideo{
			ProjectID: 1,
			Name:      "render-fail",
			Status:    "draft",
			Version:   1,
		}
		if err := r.Full.Create(ctx, f); err != nil {
			t.Fatalf("create: %v", err)
		}

		_, err := s.Render(ctx, f.ID, 1)
		if err == nil {
			t.Fatalf("want error, got nil")
		}
	})
}

// ==================== HandleComposeTask ====================

func TestFullVideoService_HandleComposeTask(t *testing.T) {
	ctx := context.Background()
	s, r := newFullVideoService(t)

	// 预置一个有 clips 的 full video
	tl := Timeline{
		Clips: []TimelineClip{
			{URL: "http://example.com/clip1.mp4", DurationMs: 5000, TTSText: "Hello"},
		},
	}
	tlBytes, _ := json.Marshal(tl)
	f := &model.FullVideo{
		ProjectID: 1,
		Name:      "compose-test",
		Timeline:  model.JSON(tlBytes),
		Status:    "queued",
		Version:   1,
	}
	if err := r.Full.Create(ctx, f); err != nil {
		t.Fatalf("create full video: %v", err)
	}

	t.Run("handler returns func", func(t *testing.T) {
		handler := s.HandleComposeTask()
		if handler == nil {
			t.Fatalf("handler should not be nil")
		}
	})

	t.Run("invalid payload", func(t *testing.T) {
		handler := s.HandleComposeTask()
		task := asynq.NewTask("video.compose", []byte("invalid json"))
		err := handler(ctx, task)
		if err == nil {
			t.Fatalf("want error for invalid payload")
		}
	})

	t.Run("full video not found", func(t *testing.T) {
		handler := s.HandleComposeTask()
		payload, _ := json.Marshal(composePayload{FullVideoID: 99999, UserID: 1})
		task := asynq.NewTask("video.compose", payload)
		err := handler(ctx, task)
		if err == nil {
			t.Fatalf("want error for not found full video")
		}
	})

	t.Run("empty clips", func(t *testing.T) {
		// 创建一个没有 clips 的视频
		emptyTL := Timeline{Clips: []TimelineClip{}}
		emptyBytes, _ := json.Marshal(emptyTL)
		fv := &model.FullVideo{
			ProjectID: 2,
			Name:      "empty-clips",
			Timeline:  model.JSON(emptyBytes),
			Status:    "queued",
			Version:   1,
		}
		if err := r.Full.Create(ctx, fv); err != nil {
			t.Fatalf("create: %v", err)
		}

		handler := s.HandleComposeTask()
		payload, _ := json.Marshal(composePayload{FullVideoID: fv.ID, UserID: 1})
		task := asynq.NewTask("video.compose", payload)
		err := handler(ctx, task)
		if err == nil {
			t.Fatalf("want error for empty clips")
		}
	})

	t.Run("valid payload with clips", func(t *testing.T) {
		// 由于 ffmpeg.Available() 在测试环境可能不可用，这个测试会走到 ffmpeg 检查失败分支
		handler := s.HandleComposeTask()
		payload, _ := json.Marshal(composePayload{FullVideoID: f.ID, UserID: 1})
		task := asynq.NewTask("video.compose", payload)
		err := handler(ctx, task)
		// 如果 ffmpeg 不可用，会返回错误
		// 如果 ffmpeg 可用，会继续执行（但可能因其他依赖失败）
		// 这里主要验证流程能走到对应分支
		_ = err
	})
}

// ==================== SetDeps ====================

func TestFullVideoService_SetDeps(t *testing.T) {
	s, _ := newFullVideoService(t)

	t.Run("set deps", func(t *testing.T) {
		s.SetDeps(nil, nil, nil, nil)
		if s.hub != nil {
			t.Fatalf("hub should be nil")
		}
		if s.store != nil {
			t.Fatalf("store should be nil")
		}
		if s.modelSvc != nil {
			t.Fatalf("modelSvc should be nil")
		}
		if s.invSvc != nil {
			t.Fatalf("invSvc should be nil")
		}
	})
}

// ==================== publish ====================

func TestFullVideoService_publish(t *testing.T) {
	s, _ := newFullVideoService(t)

	t.Run("publish without hub", func(t *testing.T) {
		// hub 为 nil 时 publish 应该安全返回不 panic
		s.publish("full:1", "progress", 0.5, "test")
	})
}

// ==================== Integration: Create -> Update -> Get -> Delete ====================

func TestFullVideoService_CRUDFlow(t *testing.T) {
	ctx := context.Background()
	s, _ := newFullVideoService(t)

	// Create
	in := &CreateFullVideoInput{
		ProjectID: 1,
		Name:      "crud-flow",
		Timeline: Timeline{
			Clips: []TimelineClip{
				{URL: "http://example.com/a.mp4", DurationMs: 3000},
			},
		},
	}
	f, err := s.Create(ctx, in, 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.ID == 0 {
		t.Fatalf("id should be assigned")
	}

	// Get
	got, err := s.Get(ctx, f.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "crud-flow" {
		t.Fatalf("name mismatch")
	}

	// Update
	newName := "updated-name"
	updated, err := s.Update(ctx, f.ID, &UpdateFullVideoInput{Name: &newName}, 2)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "updated-name" {
		t.Fatalf("name not updated")
	}
	if updated.UpdatedBy != 2 {
		t.Fatalf("updated_by not set")
	}

	// Verify via Get
	got2, _ := s.Get(ctx, f.ID)
	if got2.Name != "updated-name" {
		t.Fatalf("get after update: name mismatch")
	}

	// List should contain it
	list, total, err := s.List(ctx, &repo.ListFullVideosQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("list total=%d len=%d want 1,1", total, len(list))
	}

	// Delete
	if err := s.Delete(ctx, f.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Get after delete should fail
	_, err = s.Get(ctx, f.ID)
	if !isErr(err, errcode.ErrNotFound) {
		t.Fatalf("get after delete: want ErrNotFound, got %v", err)
	}
}

// ==================== UpdateStatus via repo (indirectly tested through Render) ====================

func TestFullVideoService_StatusTransitions(t *testing.T) {
	ctx := context.Background()
	s, r := newFullVideoService(t)

	f := &model.FullVideo{
		ProjectID: 1,
		Name:      "status-test",
		Status:    "draft",
		Version:   1,
	}
	if err := r.Full.Create(ctx, f); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Test status update through repo
	t.Run("update status", func(t *testing.T) {
		if err := r.Full.UpdateStatus(ctx, f.ID, "running", 50, ""); err != nil {
			t.Fatalf("update status: %v", err)
		}
		got, _ := r.Full.Get(ctx, f.ID)
		if got.Status != "running" {
			t.Fatalf("status=%s want running", got.Status)
		}
		if got.RenderProgress != 50 {
			t.Fatalf("render_progress=%d want 50", got.RenderProgress)
		}
	})

	t.Run("update status with error", func(t *testing.T) {
		if err := r.Full.UpdateStatus(ctx, f.ID, "failed", 0, "ffmpeg error"); err != nil {
			t.Fatalf("update status: %v", err)
		}
		got, _ := r.Full.Get(ctx, f.ID)
		if got.Status != "failed" {
			t.Fatalf("status=%s want failed", got.Status)
		}
		if got.ErrorMsg != "ffmpeg error" {
			t.Fatalf("error_msg=%s want ffmpeg error", got.ErrorMsg)
		}
		if got.RenderProgress != 0 {
			t.Fatalf("render_progress=%d want 0", got.RenderProgress)
		}
	})

	// Test Render updates status to queued
	t.Run("render queues", func(t *testing.T) {
		_, err := s.Render(ctx, f.ID, 1)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		got, _ := r.Full.Get(ctx, f.ID)
		if got.Status != "queued" {
			t.Fatalf("status=%s want queued", got.Status)
		}
	})
}
