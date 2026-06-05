package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/cache"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
	"github.com/warden-stock/warden-stock-data/pkg/response"
)

type RateLimitByCredential struct {
	limiter   cache.RateLimiter
	quota     cache.QuotaStore
	credSvc   *service.CredentialService
	accessLog *repository.AccessLogRepository
}

func NewRateLimitByCredential(
	limiter cache.RateLimiter,
	quota cache.QuotaStore,
	credSvc *service.CredentialService,
	accessLog *repository.AccessLogRepository,
) *RateLimitByCredential {
	return &RateLimitByCredential{limiter: limiter, quota: quota, credSvc: credSvc, accessLog: accessLog}
}

func (m *RateLimitByCredential) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		credID, ok := c.Get("credential_id")
		if !ok {
			c.Next()
			return
		}
		id := credID.(uint)
		cred, err := m.credSvc.GetCached(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, service.BizCode(err))
			c.Abort()
			return
		}
		allowed, err := m.limiter.Allow(c.Request.Context(), cache.RateLimitKey(id), cred.RateLimit, time.Second)
		if err != nil || !allowed {
			m.recordAccess(c, id, true)
			response.Fail(c, http.StatusTooManyRequests, errcode.ErrRateLimited)
			c.Abort()
			return
		}
		count, err := m.quota.IncrDaily(c.Request.Context(), cache.QuotaKey(id))
		if err != nil {
			c.Next()
			return
		}
		if int(count) > cred.DailyQuota {
			m.recordAccess(c, id, true)
			response.Fail(c, http.StatusTooManyRequests, errcode.ErrQuotaExceeded)
			c.Abort()
			return
		}
		c.Next()
		isError := c.Writer.Status() >= 400
		m.recordAccess(c, id, isError)
	}
}

func (m *RateLimitByCredential) recordAccess(c *gin.Context, id uint, isError bool) {
	if m.accessLog == nil {
		return
	}
	_ = m.accessLog.Incr(c.Request.Context(), id, isError)
}
