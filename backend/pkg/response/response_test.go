package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godfreygan/ai-script/backend/pkg/errcode"
	"github.com/gin-gonic/gin"
)

func setupGin() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	return c, w
}

func TestOK(t *testing.T) {
	c, w := setupGin()
	c.Set("request_id", "req-123")

	OK(c, gin.H{"foo": "bar"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body failed: %v", err)
	}
	if body.Code != 0 {
		t.Errorf("code = %d, want 0", body.Code)
	}
	if body.Message != "ok" {
		t.Errorf("message = %q, want ok", body.Message)
	}
	if body.RequestID != "req-123" {
		t.Errorf("request_id = %q, want req-123", body.RequestID)
	}
}

func TestFailWithErrcode(t *testing.T) {
	c, w := setupGin()
	c.Set("request_id", "req-456")

	Fail(c, errcode.ErrNotFound)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var body Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body failed: %v", err)
	}
	if body.Code != 40400 {
		t.Errorf("code = %d, want 40400", body.Code)
	}
	if body.Message != "资源不存在" {
		t.Errorf("message = %q, want 资源不存在", body.Message)
	}
	if body.RequestID != "req-456" {
		t.Errorf("request_id = %q, want req-456", body.RequestID)
	}
}

func TestFailWithPlainError(t *testing.T) {
	c, w := setupGin()
	c.Set("request_id", "req-789")

	Fail(c, errcode.ErrParam.Wrap(errcode.ErrParam))

	var body Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body failed: %v", err)
	}
	if body.Code != 40001 {
		t.Errorf("code = %d, want 40001", body.Code)
	}
}

func TestParsePaginationDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/list", nil)

	page, pageSize := ParsePagination(c)
	if page != 1 {
		t.Errorf("page = %d, want 1", page)
	}
	if pageSize != 20 {
		t.Errorf("pageSize = %d, want 20", pageSize)
	}
}

func TestParsePaginationCustomValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/list?page=3&page_size=50", nil)

	page, pageSize := ParsePagination(c)
	if page != 3 {
		t.Errorf("page = %d, want 3", page)
	}
	if pageSize != 50 {
		t.Errorf("pageSize = %d, want 50", pageSize)
	}
}

func TestParsePaginationPageSizeCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/list?page=1&page_size=999", nil)

	_, pageSize := ParsePagination(c)
	if pageSize != 200 {
		t.Errorf("pageSize = %d, want 200", pageSize)
	}
}

func TestParsePaginationInvalidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/list?page=0&page_size=0", nil)

	page, pageSize := ParsePagination(c)
	if page != 1 {
		t.Errorf("page = %d, want 1", page)
	}
	if pageSize != 20 {
		t.Errorf("pageSize = %d, want 20", pageSize)
	}
}

func TestMapHTTP(t *testing.T) {
	cases := []struct {
		code   int
		expect int
	}{
		{0, http.StatusOK},
		{40001, http.StatusBadRequest},
		{40101, http.StatusUnauthorized},
		{40102, http.StatusUnauthorized},
		{40301, http.StatusForbidden},
		{40400, http.StatusNotFound},
		{40901, http.StatusConflict},
		{40920, http.StatusConflict},
		{40290, http.StatusPaymentRequired},
		{40291, http.StatusPaymentRequired},
		{42900, http.StatusTooManyRequests},
		{50001, http.StatusInternalServerError},
		{99999, http.StatusInternalServerError},
	}

	for _, c := range cases {
		got := mapHTTP(c.code)
		if got != c.expect {
			t.Errorf("mapHTTP(%d) = %d, want %d", c.code, got, c.expect)
		}
	}
}
