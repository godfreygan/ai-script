package repo

import (
	"context"
	"fmt"
	"os"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var seedResources = []string{
	"user", "dept", "role", "project", "model",
	"script", "storyboard", "style", "image",
	"short_video", "full_video", "pipeline", "upload",
	"invocation", "review", "publish", "billing",
	"audit", "feature_flag",
}

var seedActions = []string{"read", "create", "update", "delete", "execute"}

var userOwnedResources = []string{
	"project", "script", "storyboard", "style", "image",
	"short_video", "full_video", "pipeline", "upload",
}

func (r *Repositories) Seed(ctx context.Context) error {
	db := r.DB.WithContext(ctx)

	adminRole, err := seedRole(db, "super_admin", "super admin", "all permissions", "all", 1)
	if err != nil {
		return fmt.Errorf("seed admin role: %w", err)
	}
	userRole, err := seedRole(db, "viewer", "viewer", "read only", "all", 1)
	if err != nil {
		return fmt.Errorf("seed user role: %w", err)
	}

	perms, err := seedPermissions(db)
	if err != nil {
		return fmt.Errorf("seed permissions: %w", err)
	}

	adminUser, err := seedAdminUser(db)
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.UserRole{
		UserID: adminUser.ID, RoleID: adminRole.ID,
	}).Error; err != nil {
		return fmt.Errorf("seed admin user_role: %w", err)
	}

	if err := seedRolePermissions(db, adminRole.ID, userRole.ID, perms); err != nil {
		return fmt.Errorf("seed role_permissions: %w", err)
	}
	if err := seedCasbinRules(db); err != nil {
		return fmt.Errorf("seed casbin rules: %w", err)
	}
	if err := seedReviewFlow(db); err != nil {
		return fmt.Errorf("seed review flow: %w", err)
	}
	if err := seedBillingQuota(db, adminUser.ID); err != nil {
		return fmt.Errorf("seed billing quota: %w", err)
	}

	return nil
}

func seedRole(db *gorm.DB, code, name, desc, dataScope string, isSystem int8) (*model.Role, error) {
	var role model.Role
	err := db.Where("code = ?", code).First(&role).Error
	if err == nil {
		if role.Status != 1 || role.IsSystem != isSystem || role.DataScope != dataScope || role.Name != name || role.Description != desc {
			updates := map[string]any{
				"status":      1,
				"is_system":   isSystem,
				"data_scope":  dataScope,
				"name":        name,
				"description": desc,
			}
			if err := db.Model(&model.Role{}).Where("id = ?", role.ID).Updates(updates).Error; err != nil {
				return nil, err
			}
			role.Status = 1
			role.IsSystem = isSystem
			role.DataScope = dataScope
			role.Name = name
			role.Description = desc
		}
		return &role, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	role = model.Role{
		Code:        code,
		Name:        name,
		Description: desc,
		DataScope:   dataScope,
		IsSystem:    isSystem,
		Status:      1,
	}
	if err := db.Create(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func seedPermissions(db *gorm.DB) ([]model.Permission, error) {
	out := make([]model.Permission, 0, len(seedResources)*len(seedActions))
	for _, res := range seedResources {
		for _, act := range seedActions {
			code := res + ":" + act
			p := model.Permission{
				Code:        code,
				Name:        code,
				Resource:    res,
				Action:      act,
				Description: fmt.Sprintf("%s %s", act, res),
			}
			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "code"}},
				DoNothing: true,
			}).Create(&p).Error; err != nil {
				return nil, err
			}
			var got model.Permission
			if err := db.Where("code = ?", code).First(&got).Error; err != nil {
				return nil, err
			}
			out = append(out, got)
		}
	}
	return out, nil
}

func seedAdminUser(db *gorm.DB) (*model.User, error) {
	pwd := os.Getenv("SEED_ADMIN_PASSWORD")
	if pwd == "" {
		pwd = "Admin@123"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var u model.User
	err = db.Where("username = ?", "admin").First(&u).Error
	if err == nil {
		updates := map[string]any{
			"status":        1,
			"password_hash": string(hash),
			"nickname":      "admin",
			"email":         "admin@example.com",
		}
		if err := db.Model(&model.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		u.Status = 1
		u.PasswordHash = string(hash)
		u.Nickname = "admin"
		u.Email = "admin@example.com"
		return &u, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	u = model.User{
		Username:     "admin",
		PasswordHash: string(hash),
		Nickname:     "admin",
		Email:        "admin@example.com",
		Status:       1,
	}
	if err := db.Create(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func seedRolePermissions(db *gorm.DB, adminRoleID, userRoleID int64, perms []model.Permission) error {
	for _, p := range perms {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.RolePermission{
			RoleID: adminRoleID, PermissionID: p.ID,
		}).Error; err != nil {
			return err
		}
	}
	ownedSet := map[string]bool{}
	for _, r := range userOwnedResources {
		ownedSet[r] = true
	}
	for _, p := range perms {
		give := p.Action == "read"
		if !give && ownedSet[p.Resource] && (p.Action == "create" || p.Action == "update") {
			give = true
		}
		if !give {
			continue
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.RolePermission{
			RoleID: userRoleID, PermissionID: p.ID,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

type casbinRule struct {
	ID    uint   `gorm:"primaryKey;autoIncrement"`
	Ptype string `gorm:"size:100"`
	V0    string `gorm:"size:100"`
	V1    string `gorm:"size:100"`
	V2    string `gorm:"size:100"`
	V3    string `gorm:"size:100"`
	V4    string `gorm:"size:100"`
	V5    string `gorm:"size:100"`
}

func (casbinRule) TableName() string { return "casbin_rule" }

func seedCasbinRules(db *gorm.DB) error {
	rules := make([]casbinRule, 0, 256)
	for _, res := range seedResources {
		for _, act := range seedActions {
			rules = append(rules, casbinRule{Ptype: "p", V0: "super_admin", V1: res, V2: act})
		}
	}
	ownedSet := map[string]bool{}
	for _, r := range userOwnedResources {
		ownedSet[r] = true
	}
	for _, res := range seedResources {
		rules = append(rules, casbinRule{Ptype: "p", V0: "viewer", V1: res, V2: "read"})
		if ownedSet[res] {
			rules = append(rules, casbinRule{Ptype: "p", V0: "viewer", V1: res, V2: "create"})
			rules = append(rules, casbinRule{Ptype: "p", V0: "viewer", V1: res, V2: "update"})
		}
	}
	for _, rule := range rules {
		var count int64
		if err := db.Table("casbin_rule").
			Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ?",
				rule.Ptype, rule.V0, rule.V1, rule.V2).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Create(&rule).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedReviewFlow(db *gorm.DB) error {
	const flowName = "default-single-step-review"
	var flow model.ReviewFlow
	err := db.Where("name = ?", flowName).First(&flow).Error
	if err == gorm.ErrRecordNotFound {
		flow = model.ReviewFlow{
			Name:        flowName,
			Description: "single step review flow for admin",
			TargetType:   "full_video",
			Enabled:      1,
			IsDefault:    1,
		}
		if err := db.Create(&flow).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	var node model.ReviewNode
	err = db.Where("flow_id = ? AND step_no = ?", flow.ID, 1).First(&node).Error
	if err == gorm.ErrRecordNotFound {
		node = model.ReviewNode{
			FlowID:           flow.ID,
			StepNo:           1,
			Name:             "admin approval",
			ApproverType:     "role",
			ApproverValue:    "super_admin",
			AllowTimeoutPass: 1,
			TimeoutHours:     24,
		}
		return db.Create(&node).Error
	}
	return err
}

func seedBillingQuota(db *gorm.DB, adminUserID int64) error {
	var q model.BillingQuota
	err := db.Where("scope_type = ? AND scope_id = ? AND metric = ? AND period = ?",
		"user", adminUserID, "invocations", "monthly").First(&q).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	q = model.BillingQuota{
		ScopeType:  "user",
		ScopeID:    adminUserID,
		Metric:     "invocations",
		Period:     "monthly",
		QuotaValue: 1000,
		Enabled:    1,
	}
	return db.Create(&q).Error
}
