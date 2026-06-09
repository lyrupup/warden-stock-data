package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
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
	// progressMu 保护 run 进度字段与 SaveRun，避免并发 worker 数据竞争。
	progressMu sync.Mutex
	// queueMu 串行化「同类型是否已有 running / 提升下一个 waiting」的决策，
	// 保证同一 job_type 同时至多只有一个 running。
	queueMu sync.Mutex
}

func NewJobRunner(updateSvc *service.UpdateService, jobRepo *repository.JobRepository) *JobRunner {
	return &JobRunner{updateSvc: updateSvc, jobRepo: jobRepo, cancel: make(map[uint]context.CancelFunc)}
}

type RunOptions struct {
	JobType string
	Market  string
	Codes   []string
}

// Submit 是排队感知的作业入口：同类型已有 running 时新作业进入 waiting 队列，
// 否则立即以 running 启动。返回创建出的 run（含最终状态）。
func (r *JobRunner) Submit(job *model.UpdateJob, opts RunOptions) (*model.UpdateJobRun, error) {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()

	ctx := context.Background()
	running, err := r.jobRepo.HasRunningByType(ctx, opts.JobType)
	if err != nil {
		return nil, err
	}
	run := &model.UpdateJobRun{
		JobID: job.ID, JobType: opts.JobType, Market: opts.Market,
		StartedAt: time.Now(),
	}
	if running {
		run.Status = "waiting"
		if err := r.jobRepo.CreateRun(ctx, run); err != nil {
			return nil, err
		}
		return run, nil
	}
	run.Status = "running"
	if err := r.jobRepo.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	go r.Run(context.Background(), job, run, opts)
	return run, nil
}

// PromoteNext 在一个作业结束后被调用：若同类型已无 running，则把队首 waiting 提升为 running 并执行。
func (r *JobRunner) PromoteNext(jobType string) {
	r.promoteNext(jobType)
}

// promoteNext 在一个作业结束后被调用：若同类型已无 running，则把队首 waiting 提升为 running 并执行。
func (r *JobRunner) promoteNext(jobType string) {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	r.promoteNextLocked(jobType)
}

// promoteNextLocked 假定已持有 queueMu；停用作业的 waiting 记录会被取消并继续尝试队首。
func (r *JobRunner) promoteNextLocked(jobType string) {
	ctx := context.Background()
	for {
		running, err := r.jobRepo.HasRunningByType(ctx, jobType)
		if err != nil || running {
			return
		}
		next, err := r.jobRepo.NextWaitingRunByType(ctx, jobType)
		if err != nil || next == nil {
			return
		}
		job, err := r.jobRepo.FindJob(ctx, next.JobID)
		if err != nil {
			return
		}
		if !job.Enabled {
			now := time.Now()
			next.Status = "canceled"
			next.FinishedAt = &now
			next.ErrorMsg = "作业已停用"
			_ = r.jobRepo.SaveRun(ctx, next)
			continue
		}
		next.Status = "running"
		next.StartedAt = time.Now()
		if err := r.jobRepo.SaveRun(ctx, next); err != nil {
			return
		}
		slog.Info("promote waiting job run", "runID", next.ID, "type", jobType)
		go r.Run(context.Background(), job, next, RunOptions{JobType: next.JobType, Market: next.Market})
		return
	}
}

// ResumeWaiting 进程启动时为每个存在 waiting 的类型尝试提升队首，恢复被中断的队列。
func (r *JobRunner) ResumeWaiting(ctx context.Context) {
	types, err := r.jobRepo.WaitingJobTypes(ctx)
	if err != nil {
		return
	}
	for _, t := range types {
		r.promoteNext(t)
	}
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

	// securities：轻量作业，仅同步证券列表（发现新股 / 改名），不逐只拉 K 线。
	if opts.JobType == JobSecurities {
		r.runSecuritiesSync(ctx, run)
		return
	}

	codes := opts.Codes
	explicitCodes := len(codes) > 0
	if len(codes) == 0 {
		secs, err := r.updateSvc.SyncSecuritiesList(ctx)
		if err == nil {
			codes = secs
		}
	}
	// 增量类型且非用户指定代码时，仅保留落后于全市场最新交易日的标的，
	// 避免对已是最新的股票重复请求数据源（全量 / 指标类型不在此过滤）。
	if !explicitCodes && (opts.JobType == "" || opts.JobType == JobKlineIncremental) {
		if pending, err := r.updateSvc.PendingCodes(ctx, codes); err == nil {
			slog.Info("incremental pending filter", "all", len(codes), "pending", len(pending))
			codes = pending
		}
	}
	run.Total = len(codes)
	_ = r.jobRepo.SaveRun(ctx, run)

	// 全市场最新交易日：用于增量更新对齐个股水位，避免停牌 / 次新股反复无效拉取。
	marketLatest, _ := r.updateSvc.MarketLatestTradeDate(ctx)

	batchSize := job.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	concurrency := job.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}
	// 全量指标回补单股内存占用大，限制并发避免峰值 RSS 过高。
	if opts.JobType == JobIndicatorFull && concurrency > 3 {
		concurrency = 3
	}

	var processed, succeeded, failed, skipped atomic.Int32
	var codesMu sync.Mutex
	failedCodes := make([]string, 0)
	skippedCodes := make([]string, 0)
	sem := make(chan struct{}, concurrency)

	for i := 0; i < len(codes); i += batchSize {
		select {
		case <-ctx.Done():
			r.finishRun(ctx, run, "canceled", &processed, &succeeded, &failed, &skipped, &codesMu, failedCodes, skippedCodes)
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
			select {
			case <-ctx.Done():
				r.finishRun(ctx, run, "canceled", &processed, &succeeded, &failed, &skipped, &codesMu, failedCodes, skippedCodes)
				return
			default:
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(code string) {
				defer wg.Done()
				defer func() { <-sem }()
				defer func() {
					if rec := recover(); rec != nil {
						if ctx.Err() != nil {
							return
						}
						failed.Add(1)
						codesMu.Lock()
						failedCodes = append(failedCodes, code)
						codesMu.Unlock()
						slog.Error("job item panic",
							"code", code,
							"panic", rec,
							"stack", string(debug.Stack()),
						)
						processed.Add(1)
						r.saveProgress(run, &processed, &succeeded, &failed, &skipped)
					}
				}()
				if err := ctx.Err(); err != nil {
					return
				}
				err := r.runOne(ctx, opts.JobType, code, marketLatest)
				if errors.Is(err, context.Canceled) {
					return
				}
				switch {
				case err == nil:
					succeeded.Add(1)
				case errors.Is(err, service.ErrNoMarketData):
					// 无行情/未上市：正常状态，计入跳过而非失败，单列代码便于运维识别。
					skipped.Add(1)
					codesMu.Lock()
					skippedCodes = append(skippedCodes, code)
					codesMu.Unlock()
					slog.Info("job item skipped (no market data)", "code", code, "err", err)
				default:
					failed.Add(1)
					codesMu.Lock()
					failedCodes = append(failedCodes, code)
					codesMu.Unlock()
					slog.Warn("job item failed", "code", code, "err", err)
				}
				processed.Add(1)
				r.saveProgress(run, &processed, &succeeded, &failed, &skipped)
			}(code)
		}
		wg.Wait()
		if ctx.Err() != nil {
			r.finishRun(ctx, run, "canceled", &processed, &succeeded, &failed, &skipped, &codesMu, failedCodes, skippedCodes)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.finishRun(ctx, run, "done", &processed, &succeeded, &failed, &skipped, &codesMu, failedCodes, skippedCodes)
}

// runSecuritiesSync 执行「仅同步证券列表」作业：调用一次 SyncSecuritiesList，
// 以同步到的证券数量作为 total/succeeded，几秒内完成，不触达 K 线接口。
func (r *JobRunner) runSecuritiesSync(ctx context.Context, run *model.UpdateJobRun) {
	var processed, succeeded, failed, skipped atomic.Int32
	codes, err := r.updateSvc.SyncSecuritiesList(ctx)
	if err != nil {
		run.ErrorMsg = err.Error()
		r.finishRun(ctx, run, "failed", &processed, &succeeded, &failed, &skipped, nil, nil, nil)
		return
	}
	n := int32(len(codes))
	run.Total = len(codes)
	processed.Store(n)
	succeeded.Store(n)
	r.finishRun(ctx, run, "done", &processed, &succeeded, &failed, &skipped, nil, nil, nil)
}

// runOne 按作业类型分发单只标的的处理逻辑。五类作业职责单一：K 线拉取与指标计算解耦。
// 旧类型 indicator/snapshot 兼容映射到「增量指标」，避免历史作业/记录失效。
func (r *JobRunner) runOne(ctx context.Context, jobType, code string, marketLatest *time.Time) error {
	switch jobType {
	case JobKlineFull:
		return r.updateSvc.SyncKlineFull(ctx, code)
	case JobIndicatorFull:
		return r.updateSvc.BackfillIndicators(ctx, code, nil)
	case JobIndicatorIncremental, "indicator", "snapshot":
		return r.updateSvc.ScanIndicators(ctx, code, nil)
	case JobKlineIncremental, "":
		return r.updateSvc.IncrementalOne(ctx, code, marketLatest)
	default:
		return r.updateSvc.IncrementalOne(ctx, code, marketLatest)
	}
}

func (r *JobRunner) saveProgress(run *model.UpdateJobRun, processed, succeeded, failed, skipped *atomic.Int32) {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	run.Processed = int(processed.Load())
	run.Succeeded = int(succeeded.Load())
	run.Failed = int(failed.Load())
	run.Skipped = int(skipped.Load())
	_ = r.jobRepo.UpdateRunProgress(context.Background(), run.ID,
		run.Processed, run.Succeeded, run.Failed, run.Skipped)
}

func (r *JobRunner) finishRun(_ context.Context, run *model.UpdateJobRun, status string, processed, succeeded, failed, skipped *atomic.Int32, codesMu *sync.Mutex, failedCodes, skippedCodes []string) {
	now := time.Now()
	if fresh, err := r.jobRepo.FindRun(context.Background(), run.ID); err == nil && fresh.Status == "canceled" {
		run.Status = "canceled"
		run.FinishedAt = fresh.FinishedAt
	} else {
		run.Status = status
		run.FinishedAt = &now
	}
	run.Processed = int(processed.Load())
	run.Succeeded = int(succeeded.Load())
	run.Failed = int(failed.Load())
	run.Skipped = int(skipped.Load())
	if codesMu != nil {
		codesMu.Lock()
		run.FailedCodes = formatFailedCodes(failedCodes)
		run.SkippedCodes = formatFailedCodes(skippedCodes)
		codesMu.Unlock()
	}
	// 终态保存使用独立 context：取消路径传入的 ctx 已 Done，会导致保存失败。
	_ = r.jobRepo.SaveRun(context.Background(), run)
	// 当前作业已落终态，接力检查同类型 waiting 队列。
	r.promoteNext(run.JobType)
}

func (r *JobRunner) Cancel(runID uint) {
	r.mu.Lock()
	if cancel, ok := r.cancel[runID]; ok {
		cancel()
	}
	r.mu.Unlock()
}
