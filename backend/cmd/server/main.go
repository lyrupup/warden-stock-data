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
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/internal/router"
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

	hash, err := crypto.HashPassword(getEnv("ADMIN_PASSWORD", "admin123"))
	if err != nil {
		slog.Error("hash admin password", "err", err)
		os.Exit(1)
	}
	if err := adminRepo.EnsureDefault(context.Background(), getEnv("ADMIN_USERNAME", "admin"), hash); err != nil {
		slog.Error("seed admin", "err", err)
		os.Exit(1)
	}
	_ = jobRepo.EnsureDefaults(context.Background())
	_ = jobRepo.EnsureDefaultDataSource(context.Background())

	provider := market.NewProvider(cfg.MarketProvider)
	quoteSvc := service.NewQuoteService(provider)
	klineSvc := service.NewKlineService(provider)
	indicatorSvc := service.NewIndicatorService(klineSvc)
	metaSvc := service.NewMetaService(quoteSvc)

	adminSvc := service.NewAdminService(adminRepo, cfg.JWTSecret, cfg.JWTExpire)
	credSvc := service.NewCredentialService(credRepo, cfg.EncKey)

	var nonceStore cache.NonceStore = cache.NewMemoryNonceStore()
	redisClient := cache.NewRedisClient(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, cfg.RedisDB)
	if err := redisClient.Ping(context.Background()).Err(); err == nil {
		nonceStore = cache.NewRedis(redisClient)
		slog.Info("redis connected for nonce store")
	} else {
		slog.Warn("redis unavailable, using in-memory nonce store", "err", err)
	}

	r := router.Setup(ginMode, router.Deps{
		AdminSvc:       adminSvc,
		CredSvc:        credSvc,
		QuoteSvc:       quoteSvc,
		KlineSvc:       klineSvc,
		IndicatorSvc:   indicatorSvc,
		MetaSvc:        metaSvc,
		JobRepo:        jobRepo,
		NonceStore:     nonceStore,
		SignSkewSec:    cfg.SignSkewSec,
		NonceTTL:       cfg.NonceTTL,
		RequestTimeout: cfg.RequestTimeout,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.AppPort),
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "port", cfg.AppPort, "env", cfg.AppEnv, "provider", cfg.MarketProvider)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
