package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/service"
)

// classifyResult 按单只标的的处理结果累计计数并归类（成功 / 无行情跳过 / 失败），
// incremental（gotdx）、full（baostock）、factors 三类作业共用，避免重复分类逻辑。
func (r *JobRunner) classifyResult(
	code string, err error,
	processed, succeeded, failed, skipped *atomic.Int32,
	codesMu *sync.Mutex, failedCodes, skippedCodes *[]string,
) {
	switch {
	case err == nil:
		succeeded.Add(1)
	case errors.Is(err, service.ErrNoMarketData):
		skipped.Add(1)
		codesMu.Lock()
		*skippedCodes = append(*skippedCodes, code)
		codesMu.Unlock()
	default:
		failed.Add(1)
		codesMu.Lock()
		*failedCodes = append(*failedCodes, code)
		codesMu.Unlock()
		slog.Warn("job item failed", "code", code, "err", err)
	}
	processed.Add(1)
}

// runIncrementalKline 执行「增量日 K」作业：走 gotdx（Go 原生并发连接池）逐只采集落库。
// gotdx 支持真并发，concurrency 个 goroutine 同时拉取，全市场当日增量分钟级完成。
func (r *JobRunner) runIncrementalKline(ctx context.Context, job *model.UpdateJob, run *model.UpdateJobRun, opts RunOptions) {
	var processed, succeeded, failed, skipped atomic.Int32
	var codesMu sync.Mutex
	failedCodes := make([]string, 0)
	skippedCodes := make([]string, 0)

	secMap, _ := r.updateSvc.SecurityMap(ctx)
	codes := opts.Codes
	explicitCodes := len(codes) > 0
	if !explicitCodes {
		codes = make([]string, 0, len(secMap))
		for c := range secMap {
			codes = append(codes, c)
		}
		sort.Strings(codes)
		if len(codes) == 0 {
			// 空库兜底：先同步证券列表再取标的全集。
			if secs, err := r.updateSvc.SyncSecuritiesList(ctx); err == nil {
				codes = secs
				secMap, _ = r.updateSvc.SecurityMap(ctx)
			}
		}
	}

	explicitRange := opts.FromDate != "" || opts.ToDate != ""
	if !explicitCodes && !explicitRange {
		if pending, err := r.updateSvc.PendingCodes(ctx, codes); err == nil {
			slog.Info("incremental pending filter", "all", len(codes), "pending", len(pending))
			codes = pending
		}
	}
	run.Total = len(codes)
	_ = r.jobRepo.SaveRun(ctx, run)

	marketLatest, _ := r.updateSvc.MarketLatestTradeDate(ctx)

	concurrency := job.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, code := range codes {
		if ctx.Err() != nil {
			break
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
					slog.Error("incremental item panic", "code", code, "panic", rec, "stack", string(debug.Stack()))
					r.classifyResult(code, errPanic(rec), &processed, &succeeded, &failed, &skipped, &codesMu, &failedCodes, &skippedCodes)
					r.saveProgress(run, &processed, &succeeded, &failed, &skipped)
				}
			}()
			if ctx.Err() != nil {
				return
			}
			var sec *model.Security
			if v, ok := secMap[code]; ok {
				sec = &v
			}
			err := r.updateSvc.IncrementalKlineGotdx(ctx, code, sec, opts.FromDate, opts.ToDate, marketLatest)
			if errors.Is(err, context.Canceled) {
				return
			}
			r.classifyResult(code, err, &processed, &succeeded, &failed, &skipped, &codesMu, &failedCodes, &skippedCodes)
			r.saveProgress(run, &processed, &succeeded, &failed, &skipped)
		}(code)
	}
	wg.Wait()

	if ctx.Err() != nil {
		r.finishRun(ctx, run, "canceled", &processed, &succeeded, &failed, &skipped, &codesMu, failedCodes, skippedCodes)
		return
	}
	r.finishRun(ctx, run, "done", &processed, &succeeded, &failed, &skipped, &codesMu, failedCodes, skippedCodes)
}

// runWeeklyAlign 执行「周级 baostock 对齐」作业（job_type=factors）：
// 按日期区间用 baostock 重新拉取全市场（或指定代码）该区间日 K，覆盖 gotdx 写入的数据
// （行 source 由 gotdx 翻为 baostock），并在同一趟刷新复权因子；执行前先刷新证券列表（ST/退市/新股）。
//
// 日期区间可选：用户/定时未指定时缺省对齐「最近 7 个交易日」（当天为交易日则含当天，见 RecentTradingRange）。
// 每只代码经 baostock 串行处理，batch_size 合批 HTTP、concurrency 控在途批次，全市场约数小时（建议周末跑）。
func (r *JobRunner) runWeeklyAlign(ctx context.Context, job *model.UpdateJob, run *model.UpdateJobRun, opts RunOptions) {
	var processed, succeeded, failed, skipped atomic.Int32
	var codesMu sync.Mutex
	failedCodes := make([]string, 0)
	skippedCodes := make([]string, 0)

	fromDate, toDate := opts.FromDate, opts.ToDate
	if fromDate == "" || toDate == "" {
		// 用户/定时未指定区间：缺省对齐「最近 7 个交易日」（当天为交易日则含当天）。
		f, t := r.updateSvc.RecentTradingRange(ctx, time.Now(), 7)
		if fromDate == "" {
			fromDate = f
		}
		if toDate == "" {
			toDate = t
		}
	}

	codes := opts.Codes
	if len(codes) == 0 {
		// 证券列表刷新（ST/退市/新股，单次 bulk）并取标的全集。
		secs, err := r.updateSvc.SyncSecuritiesList(ctx)
		if err != nil {
			run.ErrorMsg = err.Error()
			r.finishRun(ctx, run, "failed", &processed, &succeeded, &failed, &skipped, &codesMu, failedCodes, skippedCodes)
			return
		}
		codes = secs
	}
	run.Total = len(codes)
	_ = r.jobRepo.SaveRun(ctx, run)

	batchSize := job.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	concurrency := job.Concurrency
	if concurrency <= 0 {
		concurrency = 5
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
					slog.Error("weekly align batch panic", "panic", rec, "stack", string(debug.Stack()))
					for _, c := range batch {
						r.classifyResult(c, errPanic(rec), &processed, &succeeded, &failed, &skipped, &codesMu, &failedCodes, &skippedCodes)
					}
					r.saveProgress(run, &processed, &succeeded, &failed, &skipped)
				}
			}()
			if ctx.Err() != nil {
				return
			}
			// marketLatest 传 nil：对齐特定历史区间，不据全市场最新交易日推进水位（避免越过实际写入区间）。
			results, err := r.updateSvc.CollectKlineBatch(ctx, batch, "full", fromDate, toDate, nil)
			if errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				for _, c := range batch {
					r.classifyResult(c, err, &processed, &succeeded, &failed, &skipped, &codesMu, &failedCodes, &skippedCodes)
				}
				r.saveProgress(run, &processed, &succeeded, &failed, &skipped)
				return
			}
			for _, c := range batch {
				r.classifyResult(c, results[c], &processed, &succeeded, &failed, &skipped, &codesMu, &failedCodes, &skippedCodes)
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

func errPanic(rec any) error {
	return fmt.Errorf("panic: %v", rec)
}
