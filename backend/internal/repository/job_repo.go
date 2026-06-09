package repository

import (
	"context"
	"time"

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

// HasRunningByType 判断是否已有同类型作业处于 running 状态。
func (r *JobRepository) HasRunningByType(ctx context.Context, jobType string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.UpdateJobRun{}).
		Where("job_type = ? AND status = ?", jobType, "running").
		Count(&count).Error
	return count > 0, err
}

// NextWaitingRunByType 取同类型最早进入 waiting 队列的一条（FIFO）。
func (r *JobRepository) NextWaitingRunByType(ctx context.Context, jobType string) (*model.UpdateJobRun, error) {
	var run model.UpdateJobRun
	err := r.db.WithContext(ctx).
		Where("job_type = ? AND status = ?", jobType, "waiting").
		Order("id asc").First(&run).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// WaitingJobTypes 返回当前存在 waiting 作业的去重类型集合，用于启动时恢复队列。
func (r *JobRepository) WaitingJobTypes(ctx context.Context) ([]string, error) {
	var types []string
	err := r.db.WithContext(ctx).Model(&model.UpdateJobRun{}).
		Where("status = ?", "waiting").
		Distinct().Pluck("job_type", &types).Error
	return types, err
}

// MarkStaleRunningAsFailed 进程启动时把残留的 running 作业标记为 failed，
// 避免进程重启/崩溃后留下永远不会推进的孤儿 run。
func (r *JobRepository) MarkStaleRunningAsFailed(ctx context.Context) (int64, error) {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&model.UpdateJobRun{}).
		Where("status = ?", "running").
		Updates(map[string]interface{}{
			"status":      "failed",
			"error_msg":   "进程重启，作业中断",
			"finished_at": now,
		})
	return res.RowsAffected, res.Error
}

func (r *JobRepository) SaveRun(ctx context.Context, run *model.UpdateJobRun) error {
	return r.db.WithContext(ctx).Save(run).Error
}

// UpdateRunProgress 仅更新 running 作业的进度计数，避免取消后覆盖终态 status。
func (r *JobRepository) UpdateRunProgress(ctx context.Context, runID uint, processed, succeeded, failed, skipped int) error {
	return r.db.WithContext(ctx).Model(&model.UpdateJobRun{}).
		Where("id = ? AND status = ?", runID, "running").
		Updates(map[string]interface{}{
			"processed": processed,
			"succeeded": succeeded,
			"failed":    failed,
			"skipped":   skipped,
		}).Error
}

// LatestRunAt returns the most recent job run timestamp (finished_at preferred).
func (r *JobRepository) LatestRunAt(ctx context.Context) (*time.Time, error) {
	var run model.UpdateJobRun
	err := r.db.WithContext(ctx).
		Where("finished_at IS NOT NULL").
		Order("finished_at desc").
		First(&run).Error
	if err == nil && run.FinishedAt != nil {
		return run.FinishedAt, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	err = r.db.WithContext(ctx).Order("started_at desc").First(&run).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run.StartedAt, nil
}

// EnsureSchema 幂等校正作业相关表结构（存量库 init.sql 仅首次初始化执行，AutoMigrate 对
// varchar 扩宽并不总是生效）：把 job_type 扩宽到 32 以容纳 indicator_incremental 等较长类型，
// 并补齐 update_job_runs.failed_codes 列。
func (r *JobRepository) EnsureSchema(ctx context.Context) error {
	stmts := []string{
		"ALTER TABLE update_jobs ALTER COLUMN job_type TYPE varchar(32)",
		"ALTER TABLE update_job_runs ALTER COLUMN job_type TYPE varchar(32)",
		"ALTER TABLE update_job_runs ADD COLUMN IF NOT EXISTS failed_codes TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE update_job_runs ADD COLUMN IF NOT EXISTS skipped INT NOT NULL DEFAULT 0",
		"ALTER TABLE update_job_runs ADD COLUMN IF NOT EXISTS skipped_codes TEXT NOT NULL DEFAULT ''",
	}
	for _, s := range stmts {
		if err := r.db.WithContext(ctx).Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *JobRepository) EnsureDefaults(ctx context.Context) error {
	// 按 (job_type, market) 幂等补建：存量库也会自动补上后加入的默认作业。
	// 作业细分为五类，K 线拉取与指标计算解耦，盘后链路：增量日K(17:00) → 增量指标(17:30)；
	// 全量回补类（日K / 指标）开销大，默认停用，按需手动触发或单独排程。
	defaults := []model.UpdateJob{
		{
			// 证券列表同步：盘前 8:30 发现新股 / 更新名称，轻量，分批/并发参数对其无意义。
			Name: "securities-sync", JobType: "securities", Market: "CN",
			CronExpr: "0 30 8 * * *", BatchSize: 1, Concurrency: 1, Enabled: true,
		},
		{
			// 增量日K数据回补：盘后 17:00 补齐并覆盖最新一日日 K。
			Name: "daily-incremental", JobType: "incremental", Market: "CN",
			CronExpr: "0 0 17 * * *", BatchSize: 20, Concurrency: 10, Enabled: true,
		},
		{
			// 增量日K技术数据回补：盘后 17:30（增量日K之后）计算最新一日指标快照。
			Name: "daily-indicator-incremental", JobType: "indicator_incremental", Market: "CN",
			CronExpr: "0 30 17 * * *", BatchSize: 20, Concurrency: 10, Enabled: true,
		},
		{
			// 全量日K数据回补：整体覆盖回补历史日 K，开销大，默认停用，按需手动触发。
			Name: "kline-full-backfill", JobType: "full", Market: "CN",
			CronExpr: "0 0 4 * * 6", BatchSize: 20, Concurrency: 10, Enabled: false,
		},
		{
			// 全量日K技术数据回补：逐日重算全部历史指标快照，开销大，默认停用，按需手动触发。
			Name: "indicator-full-backfill", JobType: "indicator_full", Market: "CN",
			CronExpr: "0 0 6 * * 6", BatchSize: 20, Concurrency: 10, Enabled: false,
		},
	}
	for i := range defaults {
		var count int64
		if err := r.db.WithContext(ctx).Model(&model.UpdateJob{}).
			Where("job_type = ? AND market = ?", defaults[i].JobType, defaults[i].Market).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		// 显式 Select Enabled：否则 GORM 对带 default 标签的零值布尔（Enabled=false）会忽略该列，
		// 落库时被数据库默认值 TRUE 覆盖，导致「默认停用」的全量回补作业被错误启用。
		if err := r.db.WithContext(ctx).
			Select("Name", "JobType", "Market", "CronExpr", "BatchSize", "Concurrency", "Enabled").
			Create(&defaults[i]).Error; err != nil {
			return err
		}
	}
	return nil
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

// EnsureDefaultDataSource 保证存在与当前配置 provider 对应的默认数据源。
// 兼容历史库：若仅有一条旧默认源，则就地校正其 source/name，
// 同时保留 enabled/priority/config/health 等用户可调整字段。
func (r *JobRepository) EnsureDefaultDataSource(ctx context.Context, source string) error {
	if source == "" {
		source = "gotdx"
	}
	name := dataSourceName(source)

	var existing model.DataSource
	err := r.db.WithContext(ctx).Where("source = ? AND market = ?", source, "CN").First(&existing).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	var count int64
	if err := r.db.WithContext(ctx).Model(&model.DataSource{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 1 {
		var ds model.DataSource
		if err := r.db.WithContext(ctx).First(&ds).Error; err != nil {
			return err
		}
		ds.Source = source
		ds.Name = name
		return r.db.WithContext(ctx).Save(&ds).Error
	}
	return r.db.WithContext(ctx).Create(&model.DataSource{
		Source: source, Market: "CN", Name: name, Enabled: true, Priority: 0, Health: "ok",
	}).Error
}

func dataSourceName(source string) string {
	switch source {
	case "gotdx":
		return "通达信 gotdx 行情源"
	default:
		return source + " 行情源"
	}
}
