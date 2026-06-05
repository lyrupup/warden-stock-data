package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
)

// cronParser 与 cron.New(cron.WithSeconds()) 使用的解析规格保持一致（6 段含秒），
// 用于在更新作业前校验 cron 表达式格式。
var cronParser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ValidateCron 校验 cron 表达式是否符合调度器使用的 6 段（含秒）格式。
func ValidateCron(expr string) error {
	_, err := cronParser.Parse(expr)
	return err
}

type CronScheduler struct {
	cron    *cron.Cron
	runner  *JobRunner
	jobRepo *repository.JobRepository
	calRepo *repository.CalendarRepository
}

func NewCronScheduler(runner *JobRunner, jobRepo *repository.JobRepository, calRepo *repository.CalendarRepository) *CronScheduler {
	return &CronScheduler{
		cron: cron.New(cron.WithSeconds()), runner: runner,
		jobRepo: jobRepo, calRepo: calRepo,
	}
}

func (s *CronScheduler) Start(ctx context.Context) error {
	jobs, err := s.jobRepo.ListJobs(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		j := job
		_, err := s.cron.AddFunc(j.CronExpr, func() {
			s.runScheduled(ctx, &j)
		})
		if err != nil {
			slog.Warn("cron add failed", "job", j.Name, "err", err)
		}
	}
	s.cron.Start()
	return nil
}

func (s *CronScheduler) runScheduled(ctx context.Context, job *model.UpdateJob) {
	// 运行时重读配置：停用后无需重启即可跳过定时触发。
	fresh, err := s.jobRepo.FindJob(ctx, job.ID)
	if err != nil || !fresh.Enabled {
		slog.Info("skip disabled job", "job", job.Name)
		return
	}
	job = fresh

	today := time.Now()
	open, err := s.calRepo.IsTradingDay(ctx, job.Market, today)
	if err != nil || !open {
		slog.Info("skip job on non-trading day", "job", job.Name)
		return
	}
	// 走 Submit 以遵守同类型排队：若已有同类型 running，则本次进入 waiting 队列。
	if _, err := s.runner.Submit(job, RunOptions{JobType: job.JobType, Market: job.Market}); err != nil {
		slog.Warn("submit scheduled job failed", "job", job.Name, "err", err)
	}
}

func (s *CronScheduler) Stop() {
	s.cron.Stop()
}
