package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type RoleRepo struct{ db *gorm.DB }

func (r *RoleRepo) List(ctx context.Context) ([]model.Role, error) {
	var list []model.Role
	if err := r.db.WithContext(ctx).Order("id").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *RoleRepo) Get(ctx context.Context, id int64) (*model.Role, error) {
	var role model.Role
	if err := r.db.WithContext(ctx).First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepo) GetByCode(ctx context.Context, code string) (*model.Role, error) {
	var role model.Role
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepo) Create(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

func (r *RoleRepo) Update(ctx context.Context, role *model.Role) error {
	// 修复 P0 #7 — 原 Save(role) 全字段写回。改 Updates(map) 只动显式字段;
	// is_system 是建库后不应再改的标志,这里不允许通过 Update 修改,
	// 防止 Save 把零值的 is_system 反写覆盖系统角色保护标记。
	return r.db.WithContext(ctx).Model(&model.Role{}).Where("id = ?", role.ID).
		Updates(map[string]any{
			"code":        role.Code,
			"name":        role.Name,
			"description": role.Description,
			"data_scope":  role.DataScope,
			"status":      role.Status,
		}).Error
}

func (r *RoleRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.First(&role, id).Error; err != nil {
			return err
		}
		if role.IsSystem == 1 {
			return ErrConflict
		}
		var n int64
		if err := tx.Model(&model.UserRole{}).Where("role_id = ?", id).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return ErrRoleHasUsers
		}
		if err := tx.Where("role_id = ?", id).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Role{}, id).Error
	})
}

func (r *RoleRepo) ListPermissions(ctx context.Context) ([]model.Permission, error) {
	var list []model.Permission
	if err := r.db.WithContext(ctx).Order("id").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetRolePermissionCodes 拿到一个角色的权限 code 列表
func (r *RoleRepo) GetRolePermissionCodes(ctx context.Context, roleID int64) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).
		Table("role_permissions rp").
		Select("p.code").
		Joins("JOIN permissions p ON p.id = rp.permission_id").
		Where("rp.role_id = ?", roleID).
		Scan(&codes).Error
	return codes, err
}

// SetRolePermissions 替换角色的权限点(按 permission code)
func (r *RoleRepo) SetRolePermissions(ctx context.Context, roleID int64, codes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		if len(codes) == 0 {
			return nil
		}
		var perms []model.Permission
		if err := tx.Where("code IN ?", codes).Find(&perms).Error; err != nil {
			return err
		}
		rows := make([]model.RolePermission, 0, len(perms))
		for _, p := range perms {
			rows = append(rows, model.RolePermission{RoleID: roleID, PermissionID: p.ID})
		}
		if len(rows) > 0 {
			return tx.Create(&rows).Error
		}
		return nil
	})
}

// AllRolePermissions 一次性拉所有 (role_code, perm_code) 用于 Casbin 同步
func (r *RoleRepo) AllRolePermissions(ctx context.Context) ([][2]string, error) {
	type row struct {
		RoleCode string `gorm:"column:role_code"`
		PermCode string `gorm:"column:perm_code"`
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("role_permissions rp").
		Select("r.code as role_code, p.resource as perm_code, p.action as action").
		Joins("JOIN roles r ON r.id = rp.role_id AND r.deleted_at IS NULL").
		Joins("JOIN permissions p ON p.id = rp.permission_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	// scan above lost action - use raw to fetch resource+action separately
	var raw []struct {
		RoleCode string
		Resource string
		Action   string
	}
	err = r.db.WithContext(ctx).
		Table("role_permissions rp").
		Select("r.code as role_code, p.resource as resource, p.action as action").
		Joins("JOIN roles r ON r.id = rp.role_id AND r.deleted_at IS NULL").
		Joins("JOIN permissions p ON p.id = rp.permission_id").
		Scan(&raw).Error
	if err != nil {
		return nil, err
	}
	out := make([][2]string, 0, len(raw)*2)
	for _, x := range raw {
		out = append(out, [2]string{x.RoleCode, x.Resource + "|" + x.Action})
	}
	return out, nil
}
