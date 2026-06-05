package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	"github.com/warden-stock/warden-stock-data/internal/model"
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
	var req struct {
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
		job.CronExpr = *req.CronExpr
	}
	if req.BatchSize != nil {
		job.BatchSize = *req.BatchSize
	}
	if req.Concurrency != nil {
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
	run := &model.UpdateJobRun{
		JobID: job.ID, Status: "running", StartedAt: time.Now(),
	}
	if err := h.jobRepo.CreateRun(c.Request.Context(), run); err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	go h.jobRunner.Run(c.Request.Context(), job, run, scheduler.RunOptions{
		JobType: req.Type, Market: req.Market, Codes: req.Codes,
	})
	response.OK(c, gin.H{"runId": run.ID})
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
	h.jobRunner.Cancel(uint(id))
	run, err := h.jobRepo.FindRun(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, http.StatusNotFound, errcode.ErrNotFound)
		return
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
