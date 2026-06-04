package router

import (
	"time"

	"github.com/gin-gonic/gin"

	adminh "github.com/warden-stock/warden-stock-data/internal/handler/admin"
	openh "github.com/warden-stock/warden-stock-data/internal/handler/open"
	"github.com/warden-stock/warden-stock-data/internal/middleware"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/cache"
)

type Deps struct {
	AdminSvc       *service.AdminService
	CredSvc        *service.CredentialService
	QuoteSvc       *service.QuoteService
	KlineSvc       *service.KlineService
	IndicatorSvc   *service.IndicatorService
	MetaSvc        *service.MetaService
	JobRepo        *repository.JobRepository
	AccessLogRepo  *repository.AccessLogRepository
	NonceStore     cache.NonceStore
	RateLimiter    cache.RateLimiter
	QuotaStore     cache.QuotaStore
	SignSkewSec    int
	NonceTTL       time.Duration
	RequestTimeout time.Duration
}

func Setup(mode string, deps Deps) *gin.Engine {
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.Recovery(), middleware.Logger(), middleware.CORS(), middleware.Timeout(deps.RequestTimeout))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	authH := adminh.NewAuthHandler(deps.AdminSvc)
	credH := adminh.NewCredentialHandler(deps.CredSvc, deps.AccessLogRepo)
	jobH := adminh.NewJobHandler(deps.JobRepo, deps.QuoteSvc, deps.MetaSvc)
	openH := openh.NewMarketHandler(deps.QuoteSvc, deps.KlineSvc, deps.IndicatorSvc, deps.MetaSvc)

	adminAuth := middleware.NewAdminAuth(deps.AdminSvc)
	hmacAuth := middleware.NewHmacAuth(deps.CredSvc, deps.NonceStore, deps.SignSkewSec, deps.NonceTTL)

	admin := r.Group("/admin")
	{
		admin.POST("/auth/login", authH.Login)
		authed := admin.Group("", adminAuth.Middleware())
		{
			authed.GET("/auth/me", authH.Me)
			authed.GET("/credentials", credH.List)
			authed.POST("/credentials", credH.Create)
			authed.GET("/credentials/:id", credH.Get)
			authed.PUT("/credentials/:id", credH.Update)
			authed.DELETE("/credentials/:id", credH.Delete)
			authed.POST("/credentials/:id/rotate", credH.Rotate)

			authed.GET("/datasources", jobH.ListDataSources)
			authed.PUT("/datasources/:id", jobH.UpdateDataSource)
			authed.POST("/datasources/:id/healthcheck", jobH.HealthCheckDataSource)
			authed.GET("/jobs", jobH.ListJobs)
			authed.PUT("/jobs/:id", jobH.UpdateJob)
			authed.POST("/jobs/:id/run", jobH.RunJob)
			authed.GET("/jobs/runs", jobH.ListRuns)
			authed.GET("/jobs/runs/:runId", jobH.GetRun)
			authed.POST("/jobs/runs/:runId/cancel", jobH.CancelRun)
			authed.GET("/freshness", jobH.Freshness)
		}
	}

	rateLimit := middleware.NewRateLimitByCredential(deps.RateLimiter, deps.QuotaStore, deps.CredSvc, deps.AccessLogRepo)

	open := r.Group("/open/v1")
	open.Use(hmacAuth.Middleware(), rateLimit.Middleware())
	{
		open.GET("/indices", openH.Indices)
		open.GET("/quotes", openH.Quotes)
		open.GET("/stocks/:code", openH.Stock)
		open.GET("/stocks/:code/kline", openH.Kline)
		open.GET("/stocks/:code/indicators", openH.StockIndicators)
		open.GET("/indicators", openH.BatchIndicators)
		open.GET("/search", openH.Search)
		open.GET("/securities", openH.Securities)
		open.GET("/meta", openH.Meta)
	}

	return r
}
