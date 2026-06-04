package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
)

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
	today := time.Now()
	open, err := s.calRepo.IsTradingDay(ctx, job.Market, today)
	if err != nil || !open {
		slog.Info("skip job on non-trading day", "job", job.Name)
		return
	}
	run := &model.UpdateJobRun{JobID: job.ID, Status: "running", StartedAt: time.Now()}
	if err := s.jobRepo.CreateRun(ctx, run); err != nil {
		return
	}
	go s.runner.Run(ctx, job, run, RunOptions{JobType: job.JobType, Market: job.Market})
}

func (s *CronScheduler) Stop() {
	s.cron.Stop()
}
