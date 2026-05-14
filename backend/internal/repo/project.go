package repo

import (
	"context"

	"git.myscrm.cn/ganqx01/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type ProjectRepo struct{ db *gorm.DB }

type ListProjectsQuery struct {
	Page, PageSize int
	Status         int8
	Q              string
	DeptID         int64
	OwnerID        int64
	UserID         int64  // 若提供,只看该用户参与的项目
	DataScope      string // self/dept/all
}

func (r *ProjectRepo) List(ctx context.Context, q *ListProjectsQuery) ([]model.Project, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.Project{})
	if q.Status != 0 {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Q != "" {
		tx = tx.Where("name LIKE ? OR code LIKE ?", "%"+q.Q+"%", "%"+q.Q+"%")
	}
	switch q.DataScope {
	case "self":
		tx = tx.Where("owner_id = ? OR id IN (SELECT project_id FROM project_members WHERE user_id = ?)",
			q.UserID, q.UserID)
	case "dept":
		tx = tx.Where("dept_id = ?", q.DeptID)
	default: // all
	}
	if q.DeptID != 0 && q.DataScope == "" {
		tx = tx.Where("dept_id = ?", q.DeptID)
	}
	if q.OwnerID != 0 {
		tx = tx.Where("owner_id = ?", q.OwnerID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := pagination(q.Page, q.PageSize)
	var list []model.Project
	if err := tx.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *ProjectRepo) Create(ctx context.Context, p *model.Project) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *ProjectRepo) GetByID(ctx context.Context, id int64) (*model.Project, error) {
	var p model.Project
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepo) Update(ctx context.Context, p *model.Project) error {
	return r.db.WithContext(ctx).Model(&model.Project{}).Select("*").Omit("created_at").Where("id = ?", p.ID).Updates(p).Error
}

func (r *ProjectRepo) UpdateStatus(ctx context.Context, id int64, status int8) error {
	return r.db.WithContext(ctx).Model(&model.Project{}).
		Where("id = ?", id).Update("status", status).Error
}

func (r *ProjectRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Project{}, id).Error
}

// 成员管理
func (r *ProjectRepo) ListMembers(ctx context.Context, projectID int64) ([]model.ProjectMember, error) {
	var list []model.ProjectMember
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&list).Error
	return list, err
}

func (r *ProjectRepo) AddMember(ctx context.Context, m *model.ProjectMember) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *ProjectRepo) RemoveMember(ctx context.Context, projectID, userID int64) error {
	return r.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Delete(&model.ProjectMember{}).Error
}

// IsMember 项目成员 / 项目 owner 都算
func (r *ProjectRepo) IsMember(ctx context.Context, projectID, userID int64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := r.db.WithContext(ctx).Model(&model.Project{}).
		Where("id = ? AND owner_id = ?", projectID, userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
