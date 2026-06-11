package scheduler

import (
	"context"
	"errors"
	"fmt"
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
	// FromDate/ToDate 为显式回补日期区间（YYYY-MM-DD，留空表示不限）。
	// 一旦指定，按区间对目标代码统一回补，并跳过「仅落后标的」的增量预筛（PendingCodes）。
	FromDate string
	ToDate   string
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

	// calendar：轻量作业，同步 baostock 官方交易日历到 trading_calendars，不逐只拉 K 线。
	// 仅用日期区间（from/to）控制拉取范围，不涉及股票代码；to 留空时 Python 侧默认到当年年底。
	if opts.JobType == JobCalendar {
		r.runCalendarSync(ctx, run, opts.FromDate, opts.ToDate)
		return
	}

	// incremental：日常盘后增量，走 gotdx（Go 原生并发连接池），全市场分钟级完成。
	if opts.JobType == JobKlineIncremental || opts.JobType == "" {
		r.runIncrementalKline(ctx, job, run, opts)
		return
	}

	// factors：周级 baostock 对齐（必填区间），用 baostock 覆盖该区间日 K（source 翻 baostock）+刷因子+证券列表。
	if opts.JobType == JobFactors {
		r.runWeeklyAlign(ctx, job, run, opts)
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
	explicitRange := opts.FromDate != "" || opts.ToDate != ""
	// 增量类型、非用户指定代码、且未指定回补区间时，仅保留落后于全市场最新交易日的标的，
	// 避免对已是最新的股票重复请求数据源（全量 / 指标类型 / 显式区间不在此过滤）。
	// 指定了回补区间时跳过预筛：用户明确要求对全部目标代码按该区间回补。
	if !explicitCodes && !explicitRange && (opts.JobType == "" || opts.JobType == JobKlineIncremental) {
		if pending, err := r.updateSvc.PendingCodes(ctx, codes); err == nil {
			slog.Info("incremental pending filter", "all", len(codes), "pending", len(pending))
			codes = pending
		}
	}
	run.Total = len(codes)
	_ = r.jobRepo.SaveRun(ctx, run)

	// 全市场最新交易日：用于增量更新对齐个股水位，避免停牌 / 次新股反复无效拉取。
	marketLatest, _ := r.updateSvc.MarketLatestTradeDate(ctx)

	// batchSize：一次 HTTP 合批交给 Python 的代码数（baostock 在 Python 内串行处理）。
	// concurrency：同时在途的批次 HTTP 数（多 quant worker 时才真正并行，单 worker 时在 Python 锁内排队）。
	batchSize := job.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	concurrency := job.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	mode := collectModeOf(opts.JobType)

	var processed, succeeded, failed, skipped atomic.Int32
	var codesMu sync.Mutex
	failedCodes := make([]string, 0)
	skippedCodes := make([]string, 0)

	// 历史指标作业类型（indicator/snapshot）：v2 已移除快照落库，整体置为成功 no-op。
	if mode == collectModeNoop {
		n := int32(len(codes))
		processed.Store(n)
		succeeded.Store(n)
		r.finishRun(ctx, run, "done", &processed, &succeeded, &failed, &skipped, &codesMu, failedCodes, skippedCodes)
		return
	}

	classify := func(code string, err error) {
		r.classifyResult(code, err, &processed, &succeeded, &failed, &skipped, &codesMu, &failedCodes, &skippedCodes)
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := 0; i < len(codes); i += batchSize {
		if ctx.Err() != nil {
			break
		}
		end := i + batchSize
		if end > len(codes) {
			end = len(codes)
		}
		batch := codes[i:end]
		wg.Add(1)
		sem <- struct{}{}
		go func(batch []string) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if rec := recover(); rec != nil {
					if ctx.Err() != nil {
						return
					}
					slog.Error("job batch panic", "panic", rec, "stack", string(debug.Stack()))
					for _, c := range batch {
						classify(c, fmt.Errorf("panic: %v", rec))
					}
					r.saveProgress(run, &processed, &succeeded, &failed, &skipped)
				}
			}()
			if ctx.Err() != nil {
				return
			}
			results, err := r.updateSvc.CollectKlineBatch(ctx, batch, mode, opts.FromDate, opts.ToDate, marketLatest)
			if errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				// 传输级失败（网络/编码等）：整批计入失败，便于运维定位重跑。
				slog.Warn("job batch failed", "size", len(batch), "err", err)
				for _, c := range batch {
					classify(c, err)
				}
				r.saveProgress(run, &processed, &succeeded, &failed, &skipped)
				return
			}
			for _, c := range batch {
				classify(c, results[c])
			}
			r.saveProgress(run, &processed, &succeeded, &failed, &skipped)
		}(batch)
	}
	wg.Wait()

	if ctx.Err() != nil {
		r.finishRun(ctx, run, "canceled", &processed, &succeeded, &failed, &skipped, &codesMu, failedCodes, skippedCodes)
		return
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

// runCalendarSync 执行「同步交易日历」作业：调用一次 SyncCalendar 拉 baostock 官方日历入库，
// 以写入天数作为 total/succeeded，秒级完成，不触达 K 线接口。
func (r *JobRunner) runCalendarSync(ctx context.Context, run *model.UpdateJobRun, fromDate, toDate string) {
	var processed, succeeded, failed, skipped atomic.Int32
	n, err := r.updateSvc.SyncCalendar(ctx, fromDate, toDate)
	if err != nil {
		run.ErrorMsg = err.Error()
		r.finishRun(ctx, run, "failed", &processed, &succeeded, &failed, &skipped, nil, nil, nil)
		return
	}
	run.Total = n
	processed.Store(int32(n))
	succeeded.Store(int32(n))
	r.finishRun(ctx, run, "done", &processed, &succeeded, &failed, &skipped, nil, nil, nil)
}

// collectModeNoop 表示该作业类型无需采集（历史指标快照类，v2 已下线）。
const collectModeNoop = "noop"

// collectModeOf 把作业类型映射为 quant 采集模式：full / incremental / noop。
func collectModeOf(jobType string) string {
	switch jobType {
	case JobKlineFull:
		return "full"
	case JobKlineIncremental, "":
		return "incremental"
	// 历史指标作业类型：v2 已移除快照落库，视为 no-op。
	case "indicator_full", "indicator_incremental", "indicator", "snapshot":
		return collectModeNoop
	default:
		return "incremental"
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
