package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type JobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) ListJobs(ctx context.Context) ([]model.UpdateJob, error) {
	var jobs []model.UpdateJob
	err := r.db.WithContext(ctx).Order("id asc").Find(&jobs).Error
	return jobs, err
}

func (r *JobRepository) FindJob(ctx context.Context, id uint) (*model.UpdateJob, error) {
	var job model.UpdateJob
	err := r.db.WithContext(ctx).First(&job, id).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobRepository) UpdateJob(ctx context.Context, job *model.UpdateJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

func (r *JobRepository) ListRuns(ctx context.Context, page, size int) ([]model.UpdateJobRun, int64, error) {
	var runs []model.UpdateJobRun
	var total int64
	q := r.db.WithContext(ctx).Model(&model.UpdateJobRun{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * size
	err := q.Order("started_at desc").Offset(offset).Limit(size).Find(&runs).Error
	return runs, total, err
}

func (r *JobRepository) FindRun(ctx context.Context, id uint) (*model.UpdateJobRun, error) {
	var run model.UpdateJobRun
	err := r.db.WithContext(ctx).First(&run, id).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *JobRepository) CreateRun(ctx context.Context, run *model.UpdateJobRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

func (r *JobRepository) SaveRun(ctx context.Context, run *model.UpdateJobRun) error {
	return r.db.WithContext(ctx).Save(run).Error
}

func (r *JobRepository) EnsureDefaults(ctx context.Context) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.UpdateJob{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&model.UpdateJob{
		Name: "daily-incremental", JobType: "incremental", Market: "CN",
		CronExpr: "0 0 17 * * *", BatchSize: 20, Concurrency: 10, Enabled: true,
	}).Error
}

func (r *JobRepository) ListDataSources(ctx context.Context) ([]model.DataSource, error) {
	var list []model.DataSource
	err := r.db.WithContext(ctx).Order("priority asc").Find(&list).Error
	return list, err
}

func (r *JobRepository) FindDataSource(ctx context.Context, id uint) (*model.DataSource, error) {
	var ds model.DataSource
	err := r.db.WithContext(ctx).First(&ds, id).Error
	if err != nil {
		return nil, err
	}
	return &ds, nil
}

func (r *JobRepository) UpdateDataSource(ctx context.Context, ds *model.DataSource) error {
	return r.db.WithContext(ctx).Save(ds).Error
}

func (r *JobRepository) EnsureDefaultDataSource(ctx context.Context) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.DataSource{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&model.DataSource{
		Source: "stub", Market: "CN", Name: "Stub 行情源", Enabled: true, Priority: 0, Health: "ok",
	}).Error
}
