package errcode

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorString(t *testing.T) {
	e := New(40001, "参数错误")
	got := e.Error()
	want := "[40001] 参数错误"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrorStringWithWrapped(t *testing.T) {
	e := New(50001, "服务内部错误").Wrap(errors.New("db connection failed"))
	got := e.Error()
	if !strings.Contains(got, "[50001] 服务内部错误") {
		t.Errorf("Error() = %q, want contain '[50001] 服务内部错误'", got)
	}
	if !strings.Contains(got, "db connection failed") {
		t.Errorf("Error() = %q, want contain 'db connection failed'", got)
	}
}

func TestWrapDoesNotMutateOriginal(t *testing.T) {
	orig := New(40001, "参数错误")
	wrapped := orig.Wrap(errors.New("missing field"))

	if orig.Wrapped != nil {
		t.Error("original error was mutated by Wrap")
	}
	if wrapped.Wrapped == nil {
		t.Error("wrapped error should have non-nil Wrapped")
	}
	if orig.Code != wrapped.Code {
		t.Errorf("Code changed: orig=%d wrapped=%d", orig.Code, wrapped.Code)
	}
}

func TestWithMsg(t *testing.T) {
	orig := New(40001, "参数错误")
	modified := orig.WithMsg("用户名不能为空")

	if orig.Message != "参数错误" {
		t.Errorf("original Message mutated: got %q", orig.Message)
	}
	if modified.Message != "用户名不能为空" {
		t.Errorf("modified Message = %q, want 用户名不能为空", modified.Message)
	}
	if modified.Code != orig.Code {
		t.Errorf("Code changed: orig=%d modified=%d", orig.Code, modified.Code)
	}
}

func TestPredefinedErrors(t *testing.T) {
	cases := []struct {
		err  *Error
		code int
		msg  string
	}{
		{OK, 0, "ok"},
		{ErrParam, 40001, "参数错误"},
		{ErrUnauthorized, 40101, "未登录或登录已过期"},
		{ErrTokenInvalid, 40102, "token 无效"},
		{ErrForbidden, 40301, "无权限"},
		{ErrNotFound, 40400, "资源不存在"},
		{ErrConflict, 40901, "资源冲突"},
		{ErrStateInvalid, 40920, "状态不允许该操作"},
		{ErrQuotaExceeded, 40290, "额度不足"},
		{ErrRateLimit, 42900, "请求过于频繁"},
		{ErrInternal, 50001, "服务内部错误"},
		{ErrUpstreamModel, 50002, "上游模型错误"},
	}

	for _, c := range cases {
		if c.err.Code != c.code {
			t.Errorf("%s.Code = %d, want %d", c.err.Message, c.err.Code, c.code)
		}
		if c.err.Message != c.msg {
			t.Errorf("Error Message = %q, want %q", c.err.Message, c.msg)
		}
	}
}

func TestWrapNil(t *testing.T) {
	e := New(40001, "参数错误").Wrap(nil)
	if e.Wrapped != nil {
		t.Error("wrapping nil should result in nil Wrapped")
	}
}
