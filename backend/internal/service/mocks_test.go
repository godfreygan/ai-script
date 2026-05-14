package service

import (
	"errors"

	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/errcode"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newNopLog() *zap.Logger {
	return zap.NewNop()
}

// isErr 比较 errcode.Error（支持 WithMsg 后的副本）
func isErr(err, target error) bool {
	if err == nil || target == nil {
		return err == target
	}
	var e1 *errcode.Error
	var e2 *errcode.Error
	if errors.As(err, &e1) && errors.As(target, &e2) {
		return e1.Code == e2.Code
	}
	if errors.Is(err, target) {
		return true
	}
	// repo 层可能返回 gorm.ErrRecordNotFound，而 target 是 errcode.ErrNotFound
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var t *errcode.Error
		if errors.As(target, &t) && t.Code == errcode.ErrNotFound.Code {
			return true
		}
	}
	return false
}
