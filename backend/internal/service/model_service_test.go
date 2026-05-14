package service

import (
	"context"
	"testing"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/adapter"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"git.myscrm.cn/ganqx01/ai-script/backend/internal/repo"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/crypto"
	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
)

// testKeyB64 是 32 字节的 AES-256 密钥的 base64 编码
const testKeyB64 = "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY="

func newModelService(t *testing.T) (*modelService, *repo.Repositories, *crypto.Cipher) {
	db := newTestDB(t, &model.Model{})
	r := newTestRepos(db)
	cipher, err := crypto.New(testKeyB64)
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	return &modelService{r: r, cipher: cipher, registry: adapter.NewRegistry(), log: newNopLog()}, r, cipher
}

// ==================== List ====================

func TestModelService_List(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newModelService(t)

	// pre-seed models
	for i, code := range []string{"gpt-4", "gpt-3", "dall-e"} {
		mtype := "text"
		if code == "dall-e" {
			mtype = "image"
		}
		m := &model.Model{Code: code, Name: code, Type: mtype, Provider: "openai", Endpoint: "https://api.openai.com", Enabled: 1, Priority: i}
		if err := r.Model.Create(ctx, m); err != nil {
			t.Fatalf("create model: %v", err)
		}
	}

	t.Run("normal", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListModelsQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 3 {
			t.Fatalf("total=%d len=%d want 3,3", total, len(list))
		}
	})

	t.Run("filter by type", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListModelsQuery{Type: "image", Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("total=%d len=%d want 1,1", total, len(list))
		}
		if list[0].Code != "dall-e" {
			t.Fatalf("code=%s want dall-e", list[0].Code)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		list, total, err := s.List(ctx, &repo.ListModelsQuery{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if total != 3 || len(list) != 2 {
			t.Fatalf("total=%d len=%d want 3,2", total, len(list))
		}
	})
}

// ==================== Get ====================

func TestModelService_Get(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newModelService(t)

	m := &model.Model{Code: "m1", Name: "Model1", Type: "text", Provider: "p", Endpoint: "http://e", Enabled: 1}
	if err := r.Model.Create(ctx, m); err != nil {
		t.Fatalf("create model: %v", err)
	}
	// encrypt an api key
	ct, _ := s.cipher.Encrypt([]byte("secret-key"))
	if err := r.Model.UpdateAPIKey(ctx, m.ID, ct); err != nil {
		t.Fatalf("update api key: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		got, err := s.Get(ctx, m.ID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.Code != "m1" {
			t.Fatalf("code=%s want m1", got.Code)
		}
		// api key should be sanitized
		if got.APIKeyEncrypted != nil {
			t.Fatalf("api_key_encrypted should be nil")
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

func TestModelService_Create(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newModelService(t)

	t.Run("normal without api key", func(t *testing.T) {
		m, err := s.Create(ctx, &CreateModelInput{
			Code:      "m1",
			Name:      "Model1",
			Type:      "text",
			Provider:  "openai",
			Endpoint:  "https://api.openai.com",
			ModelName: "gpt-4",
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if m.Code != "m1" {
			t.Fatalf("code=%s want m1", m.Code)
		}
		if m.Enabled != 1 {
			t.Fatalf("enabled=%d want 1", m.Enabled)
		}
		// default_params should contain _model
		mp, _ := m.DefaultParams.AsMap()
		if mp["_model"] != "gpt-4" {
			t.Fatalf("default_params._model=%v want gpt-4", mp["_model"])
		}
	})

	t.Run("normal with api key", func(t *testing.T) {
		m, err := s.Create(ctx, &CreateModelInput{
			Code:     "m2",
			Name:     "Model2",
			Type:     "text",
			Provider: "openai",
			Endpoint: "https://api.openai.com",
			APIKey:   "sk-123",
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// verify encrypted in db
		dbM, err := r.Model.Get(ctx, m.ID)
		if err != nil {
			t.Fatalf("get from db: %v", err)
		}
		if len(dbM.APIKeyEncrypted) == 0 {
			t.Fatalf("api_key should be encrypted")
		}
		plain, err := s.cipher.Decrypt(dbM.APIKeyEncrypted)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if string(plain) != "sk-123" {
			t.Fatalf("api_key=%s want sk-123", string(plain))
		}
	})

	t.Run("duplicate code", func(t *testing.T) {
		_, err := s.Create(ctx, &CreateModelInput{
			Code:     "m1",
			Name:     "Dup",
			Type:     "text",
			Provider: "p",
			Endpoint: "http://e",
		})
		if !isErr(err, errcode.ErrConflict) {
			t.Fatalf("want ErrConflict, got %v", err)
		}
	})
}

// ==================== Update ====================

func TestModelService_Update(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newModelService(t)

	m := &model.Model{Code: "m1", Name: "Model1", Type: "text", Provider: "p", Endpoint: "http://e", Enabled: 1}
	if err := r.Model.Create(ctx, m); err != nil {
		t.Fatalf("create model: %v", err)
	}

	t.Run("normal update name", func(t *testing.T) {
		name := "Updated"
		updated, err := s.Update(ctx, m.ID, &UpdateModelInput{Name: &name})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated.Name != "Updated" {
			t.Fatalf("name=%s want Updated", updated.Name)
		}
	})

	t.Run("update api key", func(t *testing.T) {
		key := "new-key"
		_, err := s.Update(ctx, m.ID, &UpdateModelInput{APIKey: &key})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		dbM, err := r.Model.Get(ctx, m.ID)
		if err != nil {
			t.Fatalf("get from db: %v", err)
		}
		plain, err := s.cipher.Decrypt(dbM.APIKeyEncrypted)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if string(plain) != "new-key" {
			t.Fatalf("api_key=%s want new-key", string(plain))
		}
	})

	t.Run("not found", func(t *testing.T) {
		name := "x"
		_, err := s.Update(ctx, 99999, &UpdateModelInput{Name: &name})
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== Delete ====================

func TestModelService_Delete(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newModelService(t)

	m := &model.Model{Code: "m1", Name: "Model1", Type: "text", Provider: "p", Endpoint: "http://e", Enabled: 1}
	if err := r.Model.Create(ctx, m); err != nil {
		t.Fatalf("create model: %v", err)
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.Delete(ctx, m.ID); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		_, err := r.Model.Get(ctx, m.ID)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("model should be deleted")
		}
	})

	t.Run("not found after delete", func(t *testing.T) {
		err := s.Delete(ctx, m.ID)
		// gorm delete on non-existing record returns nil
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

// ==================== GetAdapter ====================

func TestModelService_GetAdapter(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newModelService(t)

	t.Run("disabled model", func(t *testing.T) {
		m := &model.Model{Code: "disabled", Name: "D", Type: "text", Provider: "p", Endpoint: "http://e", Enabled: 1}
		if err := r.Model.Create(ctx, m); err != nil {
			t.Fatalf("create model: %v", err)
		}
		// gorm default:1 overrides zero value on create, update explicitly
		if err := r.DB.Model(m).Update("enabled", 0).Error; err != nil {
			t.Fatalf("update enabled: %v", err)
		}
		_, gotM, err := s.GetAdapter(ctx, m.ID)
		if err == nil {
			t.Fatalf("expected error for disabled model")
		}
		if gotM == nil {
			t.Fatalf("model should be returned even when disabled")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, _, err := s.GetAdapter(ctx, 99999)
		if !isErr(err, errcode.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// ==================== LoadAllAdapters ====================

func TestModelService_LoadAllAdapters(t *testing.T) {
	ctx := context.Background()
	s, r, _ := newModelService(t)

	// seed enabled models
	for _, code := range []string{"a", "b"} {
		m := &model.Model{Code: code, Name: code, Type: "text", Provider: "p", Endpoint: "http://e", Enabled: 1}
		if err := r.Model.Create(ctx, m); err != nil {
			t.Fatalf("create model: %v", err)
		}
	}

	t.Run("normal", func(t *testing.T) {
		if err := s.LoadAllAdapters(ctx); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}
