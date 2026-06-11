package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/warden-stock/warden-stock-data/internal/config"
	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/integration/quant"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/internal/router"
	"github.com/warden-stock/warden-stock-data/internal/scheduler"
	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/cache"
	"github.com/warden-stock/warden-stock-data/pkg/crypto"
	"github.com/warden-stock/warden-stock-data/pkg/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	ginMode := gin.DebugMode
	if cfg.AppEnv == "prod" || cfg.AppEnv == "release" {
		ginMode = gin.ReleaseMode
	}

	db, err := database.Connect(database.Options{
		Host: cfg.PGHost, Port: cfg.PGPort, User: cfg.PGUser,
		Password: cfg.PGPassword, DBName: cfg.PGDB, SSLMode: cfg.PGSSLMode,
	})
	if err != nil {
		slog.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	database.AutoMigrate(db)

	adminRepo := repository.NewAdminRepository(db)
	credRepo := repository.NewCredentialRepository(db)
	jobRepo := repository.NewJobRepository(db)
	klineRepo := repository.NewKlineRepository(db)
	quoteRepo := repository.NewQuoteRepository(db)
	secRepo := repository.NewSecurityRepository(db)
	calRepo := repository.NewCalendarRepository(db)
	wmRepo := repository.NewWatermarkRepository(db)
	accessLogRepo := repository.NewAccessLogRepository(db)

	hash, err := crypto.HashPassword(getEnv("ADMIN_PASSWORD", "admin123"))
	if err != nil {
		slog.Error("hash admin password", "err", err)
		os.Exit(1)
	}
	if err := adminRepo.EnsureDefault(context.Background(), getEnv("ADMIN_USERNAME", "admin"), hash); err != nil {
		slog.Error("seed admin", "err", err)
		os.Exit(1)
	}
	if err := jobRepo.EnsureSchema(context.Background()); err != nil {
		slog.Warn("ensure job schema failed", "err", err)
	}
	_ = jobRepo.EnsureDefaults(context.Background())
	_ = jobRepo.EnsureDefaultDataSource(context.Background(), cfg.MarketProvider)
	// baostock 为日 K 全量/对齐采集源（经 Python quant 服务），登记以便运维页展示与探测。
	_ = jobRepo.EnsureDataSource(context.Background(), "baostock", "baostock 日K采集源", 1)
	if n, err := jobRepo.MarkStaleRunningAsFailed(context.Background()); err == nil && n > 0 {
		slog.Info("recovered stale running job runs", "count", n)
	}

	provider := market.NewProvider(cfg.MarketProvider)
	if c, ok := provider.(interface{ Close() error }); ok {
		defer func() { _ = c.Close() }()
	}
	quantClient := quant.NewClient(cfg.QuantBaseURL, cfg.InternalToken)

	redisClient := cache.NewRedisClient(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, cfg.RedisDB)

	var nonceStore cache.NonceStore = cache.NewMemoryNonceStore()
	var rateLimiter cache.RateLimiter = cache.NewMemoryRateLimiter()
	var quotaStore cache.QuotaStore = cache.NewMemoryQuotaStore()
	var quoteCache *cache.QuoteCache

	if err := redisClient.Ping(context.Background()).Err(); err == nil {
		nonceStore = cache.NewRedis(redisClient)
		rateLimiter = cache.NewRedisRateLimiter(redisClient)
		quotaStore = cache.NewRedisQuota(redisClient)
		quoteCache = cache.NewQuoteCache(redisClient, 30*time.Second)
		slog.Info("redis connected")
	} else {
		slog.Warn("redis unavailable, using in-memory stores", "err", err)
	}

	if err := secRepo.EnsureSearchIndexes(context.Background()); err != nil {
		slog.Warn("ensure securities search indexes failed", "err", err)
	}
	if err := klineRepo.EnsureIndexes(context.Background()); err != nil {
		slog.Warn("ensure kline indexes failed", "err", err)
	}

	quoteSvc := service.NewQuoteService(provider, quoteRepo, secRepo, quoteCache)
	klineSvc := service.NewKlineService(provider, klineRepo)
	indicatorSvc := service.NewIndicatorService(quantClient)
	metaSvc := service.NewMetaService(provider, quantClient, secRepo, klineRepo, wmRepo, jobRepo, calRepo, quoteCache)
	updateSvc := service.NewUpdateService(provider, quantClient, klineRepo, wmRepo, calRepo, secRepo)
	jobRunner := scheduler.NewJobRunner(updateSvc, jobRepo)
	cronSched := scheduler.NewCronScheduler(jobRunner, jobRepo, calRepo)

	adminSvc := service.NewAdminService(adminRepo, cfg.JWTSecret, cfg.JWTExpire)
	credSvc := service.NewCredentialService(credRepo, cfg.EncKey)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		bootstrapSecurities(ctx, secRepo, updateSvc)
		bootstrapCalendar(ctx, calRepo, updateSvc)
	}()
	jobRunner.ResumeWaiting(ctx)
	if err := cronSched.Start(ctx); err != nil {
		slog.Warn("cron start", "err", err)
	}
	defer cronSched.Stop()

	r := router.Setup(ginMode, router.Deps{
		AdminSvc:       adminSvc,
		CredSvc:        credSvc,
		QuoteSvc:       quoteSvc,
		KlineSvc:       klineSvc,
		IndicatorSvc:   indicatorSvc,
		MetaSvc:        metaSvc,
		JobRepo:        jobRepo,
		JobRunner:      jobRunner,
		AccessLogRepo:  accessLogRepo,
		NonceStore:     nonceStore,
		RateLimiter:    rateLimiter,
		QuotaStore:     quotaStore,
		SignSkewSec:    cfg.SignSkewSec,
		NonceTTL:       cfg.NonceTTL,
		RequestTimeout: cfg.RequestTimeout,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.AppPort),
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "port", cfg.AppPort, "provider", cfg.MarketProvider)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// bootstrapSecurities 首次启动时从行情源同步证券列表，避免概览页证券数量为 0。
func bootstrapSecurities(ctx context.Context, secRepo *repository.SecurityRepository, updateSvc *service.UpdateService) {
	bootCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	n, err := secRepo.Count(bootCtx, "CN")
	if err != nil || n > 0 {
		return
	}
	slog.Info("bootstrapping securities from market provider")
	if err := updateSvc.SyncSecurities(bootCtx); err != nil {
		slog.Warn("bootstrap securities failed", "err", err)
	}
}

// bootstrapCalendar 首次启动时从 baostock 拉全量交易日历，供调度器交易日感知与回测使用。
// 已有较完整日历（含休市日，远多于增量反推的零星几条）时跳过，避免每次启动重复全量拉取。
func bootstrapCalendar(ctx context.Context, calRepo *repository.CalendarRepository, updateSvc *service.UpdateService) {
	bootCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	n, err := calRepo.Count(bootCtx, "CN")
	if err != nil || n >= 100 {
		return
	}
	slog.Info("bootstrapping trading calendar from baostock")
	if days, err := updateSvc.SyncCalendar(bootCtx, "", ""); err != nil {
		slog.Warn("bootstrap calendar failed", "err", err)
	} else {
		slog.Info("trading calendar bootstrapped", "days", days)
	}
}
