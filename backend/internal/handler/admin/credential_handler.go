package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
	"github.com/warden-stock/warden-stock-data/pkg/response"
)

type CredentialHandler struct {
	credSvc   *service.CredentialService
	accessLog *repository.AccessLogRepository
}

func NewCredentialHandler(credSvc *service.CredentialService, accessLog *repository.AccessLogRepository) *CredentialHandler {
	return &CredentialHandler{credSvc: credSvc, accessLog: accessLog}
}

func (h *CredentialHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := h.credSvc.List(c.Request.Context(), page, size)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.Page(c, list, total, page, size)
}

func (h *CredentialHandler) Create(c *gin.Context) {
	var req struct {
		ConsumerName string     `json:"consumer_name" binding:"required"`
		RateLimit    int        `json:"rate_limit"`
		DailyQuota   int        `json:"daily_quota"`
		ExpireAt     *time.Time `json:"expire_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errcode.ErrParam)
		return
	}
	secret, err := h.credSvc.Create(c.Request.Context(), c.GetUint("admin_id"), req.ConsumerName, req.RateLimit, req.DailyQuota, req.ExpireAt)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, secret)
}

func (h *CredentialHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errcode.ErrParam)
		return
	}
	cred, err := h.credSvc.Get(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, http.StatusNotFound, service.BizCode(err))
		return
	}
	var logs interface{}
	if h.accessLog != nil {
		logs, _ = h.accessLog.ListByCredential(c.Request.Context(), uint(id), 30)
	}
	response.OK(c, gin.H{"credential": cred, "access_logs": logs})
}

func (h *CredentialHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errcode.ErrParam)
		return
	}
	var req struct {
		RateLimit  *int       `json:"rate_limit"`
		DailyQuota *int       `json:"daily_quota"`
		Status     *int16     `json:"status"`
		ExpireAt   *time.Time `json:"expire_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errcode.ErrParam)
		return
	}
	if err := h.credSvc.Update(c.Request.Context(), uint(id), req.RateLimit, req.DailyQuota, req.Status, req.ExpireAt); err != nil {
		response.Fail(c, http.StatusNotFound, service.BizCode(err))
		return
	}
	response.OK(c, nil)
}

func (h *CredentialHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errcode.ErrParam)
		return
	}
	if err := h.credSvc.Revoke(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, http.StatusNotFound, service.BizCode(err))
		return
	}
	response.OK(c, nil)
}

func (h *CredentialHandler) Rotate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errcode.ErrParam)
		return
	}
	secret, err := h.credSvc.Rotate(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, http.StatusNotFound, service.BizCode(err))
		return
	}
	response.OK(c, secret)
}
