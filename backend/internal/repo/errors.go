package repo

import "errors"

// 跨 repo 共享的错误
var (
	ErrDeptHasUsers  = errors.New("department has users")
	ErrRoleHasUsers  = errors.New("role has users")
	ErrProjectMember = errors.New("project member exists")
	ErrConflict      = errors.New("conflict")
)
