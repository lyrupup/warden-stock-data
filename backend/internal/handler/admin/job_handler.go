package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/internal/scheduler"
	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
	"github.com/warden-stock/warden-stock-data/pkg/response"
)

type JobHandler struct {
	jobRepo   *repository.JobRepository
	quoteSvc  *service.QuoteService
	metaSvc   *service.MetaService
	jobRunner *scheduler.JobRunner
}

func NewJobHandler(
	jobRepo *repository.JobRepository,
	quoteSvc *service.QuoteService,
	metaSvc *service.MetaService,
	jobRunner *scheduler.JobRunner,
) *JobHandler {
	return &JobHandler{jobRepo: jobRepo, quoteSvc: quoteSvc, metaSvc: metaSvc, jobRunner: jobRunner}
}

func (h *JobHandler) ListDataSources(c *gin.Context) {
	list, err := h.jobRepo.ListDataSources(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, list)
}

func (h *JobHandler) UpdateDataSource(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	ds, err := h.jobRepo.FindDataSource(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, http.StatusNotFound, errcode.ErrNotFound)
		return
	}
	var req struct {
		Enabled  *bool          `json:"enabled"`
		Priority *int           `json:"priority"`
		Config   datatypes.JSON `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errcode.ErrParam)
		return
	}
	if req.Enabled != nil {
		ds.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		ds.Priority = *req.Priority
	}
	if len(req.Config) > 0 {
		ds.Config = req.Config
	}
	if err := h.jobRepo.UpdateDataSource(c.Request.Context(), ds); err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, nil)
}

func (h *JobHandler) HealthCheckDataSource(c *gin.Context) {
	if err := h.quoteSvc.HealthCheck(c.Request.Context()); err != nil {
		response.Fail(c, http.StatusOK, errcode.ErrDataSource)
		return
	}
	response.OK(c, gin.H{"health": "ok"})
}

func (h *JobHandler) ListJobs(c *gin.Context) {
	jobs, err := h.jobRepo.ListJobs(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, jobs)
}

func (h *JobHandler) UpdateJob(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	job, err := h.jobRepo.FindJob(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, http.StatusNotFound, errcode.ErrNotFound)
		return
	}
	// job_type 与时间字段（created_at/updated_at）不可改；其余配置可改。
	var req struct {
		Name        *string `json:"name"`
		Market      *string `json:"market"`
		CronExpr    *string `json:"cron_expr"`
		BatchSize   *int    `json:"batch_size"`
		Concurrency *int    `json:"concurrency"`
		Enabled     *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errcode.ErrParam)
		return
	}
	if req.CronExpr != nil {
		if err := scheduler.ValidateCron(*req.CronExpr); err != nil {
			response.FailWithMessage(c, http.StatusBadRequest, errcode.ErrParam, "cron 表达式格式错误："+err.Error())
			return
		}
		job.CronExpr = *req.CronExpr
	}
	if req.Name != nil {
		if *req.Name == "" {
			response.FailWithMessage(c, http.StatusBadRequest, errcode.ErrParam, "作业名称不能为空")
			return
		}
		job.Name = *req.Name
	}
	if req.Market != nil {
		job.Market = *req.Market
	}
	if req.BatchSize != nil {
		if *req.BatchSize <= 0 {
			response.FailWithMessage(c, http.StatusBadRequest, errcode.ErrParam, "分批大小必须大于 0")
			return
		}
		job.BatchSize = *req.BatchSize
	}
	if req.Concurrency != nil {
		if *req.Concurrency <= 0 {
			response.FailWithMessage(c, http.StatusBadRequest, errcode.ErrParam, "并发数必须大于 0")
			return
		}
		job.Concurrency = *req.Concurrency
	}
	if req.Enabled != nil {
		job.Enabled = *req.Enabled
	}
	if err := h.jobRepo.UpdateJob(c.Request.Context(), job); err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, nil)
}

func (h *JobHandler) RunJob(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	job, err := h.jobRepo.FindJob(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, http.StatusNotFound, errcode.ErrNotFound)
		return
	}
	var req struct {
		Type   string   `json:"type"`
		Market string   `json:"market"`
		Codes  []string `json:"codes"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Type == "" {
		req.Type = job.JobType
	}
	if req.Market == "" {
		req.Market = job.Market
	}
	if !job.Enabled {
		response.FailWithMessage(c, http.StatusBadRequest, errcode.ErrParam, "作业已停用，无法手动触发")
		return
	}
	// Submit 内部完成「同类型已有 running → 进入 waiting，否则立即 running」的排队决策，
	// 并以独立 context 异步执行（HTTP 请求 context 会在响应后取消）。
	run, err := h.jobRunner.Submit(job, scheduler.RunOptions{
		JobType: req.Type, Market: req.Market, Codes: req.Codes,
	})
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, gin.H{"runId": run.ID, "status": run.Status})
}

func (h *JobHandler) ListRuns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	runs, total, err := h.jobRepo.ListRuns(c.Request.Context(), page, size)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.Page(c, runs, total, page, size)
}

func (h *JobHandler) GetRun(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("runId"), 10, 64)
	run, err := h.jobRepo.FindRun(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, http.StatusNotFound, errcode.ErrNotFound)
		return
	}
	response.OK(c, run)
}

func (h *JobHandler) CancelRun(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("runId"), 10, 64)
	run, err := h.jobRepo.FindRun(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, http.StatusNotFound, errcode.ErrNotFound)
		return
	}
	// 已是终态则幂等返回，避免覆盖已完成记录。
	if run.Status != "running" && run.Status != "waiting" {
		response.OK(c, nil)
		return
	}
	// running：发出取消信号，worker 检测 ctx 后立即退出，Run 落终态并接力 waiting。
	// waiting：无执行 goroutine，直接置为 canceled 并手动接力队列。
	if run.Status == "running" {
		h.jobRunner.Cancel(uint(id))
	} else {
		h.jobRunner.PromoteNext(run.JobType)
	}
	now := time.Now()
	run.Status = "canceled"
	run.FinishedAt = &now
	_ = h.jobRepo.SaveRun(c.Request.Context(), run)
	response.OK(c, nil)
}

func (h *JobHandler) Freshness(c *gin.Context) {
	market := c.DefaultQuery("market", "CN")
	f, err := h.metaSvc.Freshness(c.Request.Context(), market)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, f)
}
