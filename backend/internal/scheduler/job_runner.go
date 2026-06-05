package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/internal/service"
)

type JobRunner struct {
	updateSvc *service.UpdateService
	jobRepo   *repository.JobRepository
	mu        sync.Mutex
	cancel    map[uint]context.CancelFunc
}

func NewJobRunner(updateSvc *service.UpdateService, jobRepo *repository.JobRepository) *JobRunner {
	return &JobRunner{updateSvc: updateSvc, jobRepo: jobRepo, cancel: make(map[uint]context.CancelFunc)}
}

type RunOptions struct {
	JobType string
	Market  string
	Codes   []string
}

func (r *JobRunner) Run(ctx context.Context, job *model.UpdateJob, run *model.UpdateJobRun, opts RunOptions) {
	ctx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.cancel[run.ID] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.cancel, run.ID)
		r.mu.Unlock()
		cancel()
	}()

	codes := opts.Codes
	if len(codes) == 0 {
		secs, err := r.updateSvc.SyncSecuritiesList(ctx)
		if err == nil {
			codes = secs
		}
	}
	run.Total = len(codes)
	_ = r.jobRepo.SaveRun(ctx, run)

	batchSize := job.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	concurrency := job.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	var processed, succeeded, failed atomic.Int32
	sem := make(chan struct{}, concurrency)

	for i := 0; i < len(codes); i += batchSize {
		select {
		case <-ctx.Done():
			r.finishRun(ctx, run, "canceled", &processed, &succeeded, &failed)
			return
		default:
		}
		end := i + batchSize
		if end > len(codes) {
			end = len(codes)
		}
		batch := codes[i:end]
		var wg sync.WaitGroup
		for _, code := range batch {
			wg.Add(1)
			sem <- struct{}{}
			go func(code string) {
				defer wg.Done()
				defer func() { <-sem }()
				if err := r.runOne(ctx, opts.JobType, code); err != nil {
					failed.Add(1)
					slog.Warn("job item failed", "code", code, "err", err)
				} else {
					succeeded.Add(1)
				}
				processed.Add(1)
				run.Processed = int(processed.Load())
				run.Succeeded = int(succeeded.Load())
				run.Failed = int(failed.Load())
				_ = r.jobRepo.SaveRun(ctx, run)
			}(code)
		}
		wg.Wait()
		time.Sleep(100 * time.Millisecond)
	}
	r.finishRun(ctx, run, "done", &processed, &succeeded, &failed)
}

func (r *JobRunner) runOne(ctx context.Context, jobType, code string) error {
	switch jobType {
	case "indicator", "snapshot":
		return r.updateSvc.ScanIndicators(ctx, code, nil)
	case "full":
		if err := r.updateSvc.IncrementalOne(ctx, code); err != nil {
			return err
		}
		return r.updateSvc.BackfillIndicators(ctx, code, nil)
	default:
		if err := r.updateSvc.IncrementalOne(ctx, code); err != nil {
			return err
		}
		return r.updateSvc.ScanIndicators(ctx, code, nil)
	}
}

func (r *JobRunner) finishRun(ctx context.Context, run *model.UpdateJobRun, status string, processed, succeeded, failed *atomic.Int32) {
	now := time.Now()
	run.Status = status
	run.Processed = int(processed.Load())
	run.Succeeded = int(succeeded.Load())
	run.Failed = int(failed.Load())
	run.FinishedAt = &now
	_ = r.jobRepo.SaveRun(ctx, run)
}

func (r *JobRunner) Cancel(runID uint) {
	r.mu.Lock()
	if cancel, ok := r.cancel[runID]; ok {
		cancel()
	}
	r.mu.Unlock()
}
