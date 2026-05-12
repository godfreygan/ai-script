package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type UserRepo struct{ db *gorm.DB }

type ListUsersQuery struct {
	Page, PageSize int
	Q              string
	DeptID         int64
	Status         int8
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *UserRepo) Update(ctx context.Context, u *model.User) error {
	// 修复 P0 #7 — 原 Save(u) 会把 password_hash / last_login_at 等独立通道
	// 维护的字段一起写回。改 Updates(map) 只动用户 profile 字段;敏感字段
	// 由 UpdatePassword / UpdateLastLogin 独立 SQL 维护,避免并发覆盖。
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", u.ID).
		Updates(map[string]any{
			"username":   u.Username,
			"nickname":   u.Nickname,
			"email":      u.Email,
			"phone":      u.Phone,
			"avatar_url": u.AvatarURL,
			"dept_id":    u.DeptID,
			"status":     u.Status,
		}).Error
}

func (r *UserRepo) UpdatePassword(ctx context.Context, id int64, hash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("password_hash", hash).Error
}

func (r *UserRepo) UpdateLastLogin(ctx context.Context, id int64, ip string) error {
	return r.db.WithContext(ctx).Exec(
		"UPDATE users SET last_login_at = NOW(3), last_login_ip = ? WHERE id = ?", ip, id,
	).Error
}

func (r *UserRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, id).Error
}

func (r *UserRepo) List(ctx context.Context, q *ListUsersQuery) ([]model.User, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.User{})
	if q.Q != "" {
		tx = tx.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ?",
			"%"+q.Q+"%", "%"+q.Q+"%", "%"+q.Q+"%")
	}
	if q.DeptID != 0 {
		tx = tx.Where("dept_id = ?", q.DeptID)
	}
	if q.Status != 0 {
		tx = tx.Where("status = ?", q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.User
	if err := tx.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// GetRoleCodes 返回用户的角色 code 列表(用于 JWT.roles + Casbin sub)
func (r *UserRepo) GetRoleCodes(ctx context.Context, userID int64) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).
		Table("user_roles ur").
		Select("r.code").
		Joins("JOIN roles r ON r.id = ur.role_id AND r.deleted_at IS NULL").
		Where("ur.user_id = ? AND r.status = 1", userID).
		Scan(&codes).Error
	return codes, err
}

func (r *UserRepo) SetRoles(ctx context.Context, userID int64, roleIDs []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]model.UserRole, 0, len(roleIDs))
		for _, rid := range roleIDs {
			rows = append(rows, model.UserRole{UserID: userID, RoleID: rid})
		}
		return tx.Create(&rows).Error
	})
}

func (r *UserRepo) GetUserRoles(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Model(&model.UserRole{}).
		Where("user_id = ?", userID).Pluck("role_id", &ids).Error
	return ids, err
}

func pagination(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}
