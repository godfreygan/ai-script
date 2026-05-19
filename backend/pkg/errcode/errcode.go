package errcode

import "fmt"

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Wrapped error  `json:"-"`
}

func (e *Error) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Wrapped)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Code == t.Code
	}
	return false
}

func (e *Error) Wrap(err error) *Error {
	cp := *e
	cp.Wrapped = err
	return &cp
}

// WithMsg 复制当前错误并替换 Message,便于在同一错误码下携带不同的提示文案。
func (e *Error) WithMsg(msg string) *Error {
	cp := *e
	cp.Message = msg
	return &cp
}

func New(code int, msg string) *Error { return &Error{Code: code, Message: msg} }

// 预定义错误
var (
	OK                    = New(0, "ok")
	ErrParam              = New(40001, "参数错误")
	ErrInvalidParam       = New(40002, "请求参数无效")
	ErrUnauthorized       = New(40101, "未登录或登录已过期")
	ErrTokenInvalid       = New(40102, "token 无效")
	ErrAccountDisabled    = New(40103, "账户已被禁用")
	ErrInvalidCredentials = New(40104, "用户名或密码错误")
	ErrForbidden          = New(40301, "无权限")
	ErrNotFound           = New(40400, "资源不存在")
	ErrMethodNotAllowed   = New(40500, "方法不允许")
	ErrConflict           = New(40901, "资源冲突")
	ErrStateInvalid       = New(40920, "状态不允许该操作")
	ErrQuotaExceeded      = New(40290, "额度不足")
	ErrRateLimit          = New(42900, "请求过于频繁")
	ErrAccountLocked      = New(42301, "账户已被锁定")
	ErrWeakPassword       = New(40003, "密码强度不足")
	ErrInternal           = New(50001, "服务内部错误")
	ErrUpstreamModel      = New(50002, "上游模型错误")
	ErrTimeout            = New(50401, "请求超时")
)
