package repo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// seedResources 所有受权限控制的资源
var seedResources = []string{
	"user", "dept", "role", "project", "model",
	"script", "storyboard", "style", "image",
	"short_video", "full_video", "pipeline", "upload",
	"invocation", "review", "publish", "billing",
	"audit", "feature_flag",
}

// seedActions 所有动作
var seedActions = []string{"read", "create", "update", "delete", "execute"}

// userOwnedResources 普通用户可对自己资源执行 create/update 的资源集合
var userOwnedResources = []string{
	"project", "script", "storyboard", "style", "image",
	"short_video", "full_video", "pipeline", "upload",
}

// Seed 写入默认初始化数据。必须保证幂等(可以重复执行不出错)。
//
// 顺序:
//  1. admin / user 角色
//  2. 所有 permission(resource × action)
//  3. 默认 admin 用户 + 绑定 admin 角色
//  4. role_permissions:admin 拥有全部权限,user 拥有 read + 自己资源的 create/update
//  5. casbin_rule:admin 全开;user 全部 read + userOwnedResources 的 create/update
//  6. 默认审核流 + 单节点
//  7. admin 用户的默认 BillingQuota
func (r *Repositories) Seed(ctx context.Context) error {
	db := r.DB.WithContext(ctx)

	// 1. 角色
	adminRole, err := seedRole(db, "super_admin", "超级管理员", "拥有系统全部权限", "all", 1)
	if err != nil {
		return fmt.Errorf("seed admin role: %w", err)
	}
	userRole, err := seedRole(db, "viewer", "访客", "只读已发布", "all", 1)
	if err != nil {
		return fmt.Errorf("seed user role: %w", err)
	}

	// 2. 权限点
	perms, err := seedPermissions(db)
	if err != nil {
		return fmt.Errorf("seed permissions: %w", err)
	}

	// 3. admin 用户
	adminUser, err := seedAdminUser(db)
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}

	// 3.1 admin → admin 角色
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.UserRole{
		UserID: adminUser.ID, RoleID: adminRole.ID,
	}).Error; err != nil {
		return fmt.Errorf("seed admin user_role: %w", err)
	}

	// 4. role_permissions
	if err := seedRolePermissions(db, adminRole.ID, userRole.ID, perms); err != nil {
		return fmt.Errorf("seed role_permissions: %w", err)
	}

	// 5. casbin_rule(直接 ON DUPLICATE KEY → DoNothing,SyncCasbin 之后还会重写一次)
	if err := seedCasbinRules(db); err != nil {
		return fmt.Errorf("seed casbin rules: %w", err)
	}

	// 6. 默认审核流
	if err := seedReviewFlow(db); err != nil {
		return fmt.Errorf("seed review flow: %w", err)
	}

	// 7. 默认 BillingQuota
	if err := seedBillingQuota(db, adminUser.ID); err != nil {
		return fmt.Errorf("seed billing quota: %w", err)
	}

	return nil
}

func seedRole(db *gorm.DB, code, name, desc, dataScope string, isSystem int8) (*model.Role, error) {
	var role model.Role
	err := db.Where("code = ?", code).First(&role).Error
	if err == nil {
		return &role, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	role = model.Role{
		Code: code, Name: name, Description: desc,
		DataScope: dataScope, IsSystem: isSystem, Status: 1,
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
				Code: code, Name: code, Resource: res, Action: act,
				Description: fmt.Sprintf("%s %s", act, res),
			}
			// uniqueIndex on Code → OnConflict DoNothing 保持幂等
			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "code"}},
				DoNothing: true,
			}).Create(&p).Error; err != nil {
				return nil, err
			}
			// 重新取一次拿到 ID(insert ignore 时 GORM 不会回填)
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
	var u model.User
	err := db.Where("username = ?", "admin").First(&u).Error
	if err == nil {
		return &u, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	pwd := os.Getenv("SEED_ADMIN_PASSWORD")
	if pwd == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generate random password: %w", err)
		}
		pwd = hex.EncodeToString(b)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u = model.User{
		Username:     "admin",
		PasswordHash: string(hash),
		Nickname:     "管理员",
		Email:        "admin@example.com",
		Status:       1,
	}
	if err := db.Create(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func seedRolePermissions(db *gorm.DB, adminRoleID, userRoleID int64, perms []model.Permission) error {
	// admin → 所有权限
	for _, p := range perms {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.RolePermission{
			RoleID: adminRoleID, PermissionID: p.ID,
		}).Error; err != nil {
			return err
		}
	}
	// user → 所有 read + userOwnedResources 的 create/update
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

// casbinRule 对应 GORM AutoMigrate / casbin gorm-adapter 创建的 casbin_rule 表结构
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
	// 表是 gorm-adapter 自己建的,这里直接写 (ptype=p, v0=role, v1=resource, v2=action)
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
	// gorm-adapter 没有 unique index,这里先用 ptype+v0+v1+v2 去重检查再 insert,保持幂等
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
	const flowName = "默认单节点审核"
	var flow model.ReviewFlow
	err := db.Where("name = ?", flowName).First(&flow).Error
	if err == gorm.ErrRecordNotFound {
		flow = model.ReviewFlow{
			Name:        flowName,
			Description: "系统默认的单节点审核流:admin 角色审批,超时自动通过",
			TargetType:  "full_video",
			Enabled:     1,
			IsDefault:   1,
		}
		if err := db.Create(&flow).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// 节点
	var node model.ReviewNode
	err = db.Where("flow_id = ? AND step_no = ?", flow.ID, 1).First(&node).Error
	if err == gorm.ErrRecordNotFound {
		node = model.ReviewNode{
			FlowID:           flow.ID,
			StepNo:           1,
			Name:             "管理员审核",
			ApproverType:     "role",
			ApproverValue:    "admin",
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
