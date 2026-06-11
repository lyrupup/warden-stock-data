package middleware

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/cache"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
	"github.com/warden-stock/warden-stock-data/pkg/response"
	"github.com/warden-stock/warden-stock-data/pkg/signature"
)

func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(start),
		)
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Secret-Id,X-Timestamp,X-Nonce,X-Signature")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

type AdminAuth struct {
	adminSvc *service.AdminService
}

func NewAdminAuth(adminSvc *service.AdminService) *AdminAuth {
	return &AdminAuth{adminSvc: adminSvc}
}

func (m *AdminAuth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if len(auth) < 8 || auth[:7] != "Bearer " {
			response.Fail(c, http.StatusUnauthorized, errcode.ErrAdminUnauthorized)
			c.Abort()
			return
		}
		adminID, err := m.adminSvc.ParseToken(auth[7:])
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, service.BizCode(err))
			c.Abort()
			return
		}
		c.Set("admin_id", adminID)
		c.Next()
	}
}

type HmacAuth struct {
	credSvc  *service.CredentialService
	nonce    cache.NonceStore
	skewSec  int
	nonceTTL time.Duration
}

func NewHmacAuth(credSvc *service.CredentialService, nonce cache.NonceStore, skewSec int, nonceTTL time.Duration) *HmacAuth {
	return &HmacAuth{credSvc: credSvc, nonce: nonce, skewSec: skewSec, nonceTTL: nonceTTL}
}

func (m *HmacAuth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		secretID := c.GetHeader("X-Secret-Id")
		ts := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")
		sig := c.GetHeader("X-Signature")
		if secretID == "" || ts == "" || nonce == "" || sig == "" {
			response.Fail(c, http.StatusUnauthorized, errcode.ErrMissingSignature)
			c.Abort()
			return
		}
		tsMillis, err := strconv.ParseInt(ts, 10, 64)
		if err != nil || !signature.IsTimestampValid(tsMillis, m.skewSec) {
			response.Fail(c, http.StatusUnauthorized, errcode.ErrReplayOrExpired)
			c.Abort()
			return
		}
		nonceKey := "warden:nonce:" + secretID + ":" + nonce
		ok, err := m.nonce.SetNX(c.Request.Context(), nonceKey, m.nonceTTL)
		if err != nil || !ok {
			response.Fail(c, http.StatusUnauthorized, errcode.ErrReplayOrExpired)
			c.Abort()
			return
		}
		secretKey, cred, err := m.credSvc.ResolveSecretKeyCached(c.Request.Context(), secretID)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, service.BizCode(err))
			c.Abort()
			return
		}
		if cred.Scope != "read" {
			response.Fail(c, http.StatusForbidden, errcode.ErrScopeInsufficient)
			c.Abort()
			return
		}
		body, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		stringToSign := signature.BuildStringToSign(
			c.Request.Method,
			c.Request.URL.Path,
			signature.CanonicalQuery(c.Request.URL.Query()),
			secretID, ts, nonce,
			body,
		)
		if !signature.Verify(secretKey, stringToSign, sig) {
			response.Fail(c, http.StatusUnauthorized, errcode.ErrSignatureMismatch)
			c.Abort()
			return
		}
		c.Set("credential_id", cred.ID)
		c.Next()
	}
}
