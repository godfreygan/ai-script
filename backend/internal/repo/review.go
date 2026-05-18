package repo

import (
	"context"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/model"
	"gorm.io/gorm"
)

type ReviewRepo struct{ db *gorm.DB }

func (r *ReviewRepo) ListFlows(ctx context.Context) ([]model.ReviewFlow, error) {
	var list []model.ReviewFlow
	err := r.db.WithContext(ctx).Where("enabled = 1").Order("id").Find(&list).Error
	return list, err
}

func (r *ReviewRepo) GetFlow(ctx context.Context, id int64) (*model.ReviewFlow, error) {
	var f model.ReviewFlow
	if err := r.db.WithContext(ctx).First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *ReviewRepo) DefaultFlow(ctx context.Context, targetType string) (*model.ReviewFlow, error) {
	var f model.ReviewFlow
	err := r.db.WithContext(ctx).
		Where("target_type = ? AND is_default = 1 AND enabled = 1", targetType).
		First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *ReviewRepo) ListNodes(ctx context.Context, flowID int64) ([]model.ReviewNode, error) {
	var list []model.ReviewNode
	err := r.db.WithContext(ctx).Where("flow_id = ?", flowID).Order("step_no").Find(&list).Error
	return list, err
}

func (r *ReviewRepo) CreateRecord(ctx context.Context, rec *model.ReviewRecord) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *ReviewRepo) GetRecord(ctx context.Context, id int64) (*model.ReviewRecord, error) {
	var rec model.ReviewRecord
	if err := r.db.WithContext(ctx).First(&rec, id).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *ReviewRepo) GetActiveRecord(ctx context.Context, targetType string, targetID int64) (*model.ReviewRecord, error) {
	var rec model.ReviewRecord
	err := r.db.WithContext(ctx).Where("target_type = ? AND target_id = ? AND status = 'pending'",
		targetType, targetID).First(&rec).Error
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *ReviewRepo) ListRecords(ctx context.Context, status string, page, size int) ([]model.ReviewRecord, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.ReviewRecord{})
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	p, s := pagination(page, size)
	var list []model.ReviewRecord
	if err := tx.Order("id desc").Offset((p - 1) * s).Limit(s).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *ReviewRepo) UpdateRecord(ctx context.Context, id int64, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.ReviewRecord{}).Where("id = ?", id).Updates(fields).Error
}

// UpdateRecordCAS 用 SQL 行锁做真 CAS:WHERE 子句包含 expectStatus + expectStep,
// 由 MySQL 保证读改一体。RowsAffected=0 视为版本冲突(其他 reviewer 已抢先推进/撤回)。
//
// 修复 P0 #6 — 原 service.Act 的 fresh.Status 校验 + UpdateRecord 是读后写两步,
// 中间存在 race window:两个 reviewer 同时 approve,都看到 status=pending、
// current_step=2,会先后写入 current_step=3 与 status=approved,产生重复推进。
func (r *ReviewRepo) UpdateRecordCAS(
	ctx context.Context,
	id int64,
	expectStatus string,
	expectStep int,
	fields map[string]any,
) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.ReviewRecord{}).
		Where("id = ? AND status = ? AND current_step = ?", id, expectStatus, expectStep).
		Updates(fields)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *ReviewRepo) AddNodeRecord(ctx context.Context, nr *model.ReviewNodeRecord) error {
	nr.ActedAt = time.Now()
	return r.db.WithContext(ctx).Create(nr).Error
}

func (r *ReviewRepo) ListNodeRecords(ctx context.Context, recordID int64) ([]model.ReviewNodeRecord, error) {
	var list []model.ReviewNodeRecord
	err := r.db.WithContext(ctx).Where("review_record_id = ?", recordID).Order("id asc").Find(&list).Error
	return list, err
}

// =============== Publish ===============

type PublishRepo struct{ db *gorm.DB }

func (r *PublishRepo) Upsert(ctx context.Context, p *model.Publish) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exist model.Publish
		err := tx.Where("full_video_id = ?", p.FullVideoID).First(&exist).Error
		if err == nil {
			p.ID = exist.ID
			return tx.Model(&model.Publish{}).Where("id = ?", p.ID).
				Updates(map[string]any{
					"full_video_id":    p.FullVideoID,
					"published_by":     p.PublishedBy,
					"published_at":     p.PublishedAt,
					"status":           p.Status,
					"watermark_config": p.WatermarkConfig,
					"download_count":   p.DownloadCount,
					"play_count":       p.PlayCount,
				}).Error
		}
		return tx.Create(p).Error
	})
}

func (r *PublishRepo) GetByVideoID(ctx context.Context, videoID int64) (*model.Publish, error) {
	var p model.Publish
	if err := r.db.WithContext(ctx).Where("full_video_id = ?", videoID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PublishRepo) List(ctx context.Context, status string, page, size int) ([]model.Publish, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.Publish{})
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	p, s := pagination(page, size)
	var list []model.Publish
	if err := tx.Order("published_at desc").Offset((p - 1) * s).Limit(s).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *PublishRepo) IncPlayCount(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&model.Publish{}).Where("id = ?", id).
		UpdateColumn("play_count", gorm.Expr("play_count + 1")).Error
}

func (r *PublishRepo) IncDownloadCount(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&model.Publish{}).Where("id = ?", id).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
}

func (r *PublishRepo) SetStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).Model(&model.Publish{}).Where("id = ?", id).
		Update("status", status).Error
}
