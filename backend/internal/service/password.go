package service

import (
	"strings"
	"unicode"

	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
)

var (
	errPasswordTooShort         = errcode.ErrParam.WithMsg("密码长度不能少于8位")
	errPasswordWeak             = errcode.ErrWeakPassword.WithMsg("密码复杂度不足，需同时包含大写字母、小写字母、数字、特殊字符中的至少三种")
	errPasswordContainsUsername = errcode.ErrParam.WithMsg("密码不能包含用户名")
)

// ValidatePassword 校验密码复杂度。
// 规则：
//   - 最小长度 8 位
//   - 至少包含大写字母、小写字母、数字、特殊字符中的三种
//   - 不能包含用户名（大小写不敏感）
func ValidatePassword(password, username string) error {
	if len(password) < 8 {
		return errPasswordTooShort
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	variety := 0
	if hasUpper {
		variety++
	}
	if hasLower {
		variety++
	}
	if hasDigit {
		variety++
	}
	if hasSpecial {
		variety++
	}
	if variety < 3 {
		return errPasswordWeak
	}

	if username != "" && strings.Contains(strings.ToLower(password), strings.ToLower(username)) {
		return errPasswordContainsUsername
	}

	return nil
}

// IsWeakPassword 是 ValidatePassword 的布尔封装，用于仅做检测不返回具体错误。
func IsWeakPassword(password, username string) bool {
	return ValidatePassword(password, username) != nil
}
