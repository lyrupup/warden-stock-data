package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"github.com/warden-stock/warden-stock-data/internal/config"
	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/database"
	"github.com/warden-stock/warden-stock-data/pkg/utils"
)

func main() {
	years := flag.Int("years", 5, "years of history to backfill")
	codes := flag.String("codes", "", "comma-separated stock codes, empty = all from provider")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	db, err := database.Connect(database.Options{
		Host: cfg.PGHost, Port: cfg.PGPort, User: cfg.PGUser,
		Password: cfg.PGPassword, DBName: cfg.PGDB, SSLMode: cfg.PGSSLMode,
	})
	if err != nil {
		slog.Error("db", "err", err)
		os.Exit(1)
	}
	database.AutoMigrate(db)

	provider := market.NewProvider(cfg.MarketProvider)
	klineRepo := repository.NewKlineRepository(db)
	wmRepo := repository.NewWatermarkRepository(db)
	calRepo := repository.NewCalendarRepository(db)
	secRepo := repository.NewSecurityRepository(db)
	indiRepo := repository.NewIndicatorRepository(db)
	klineSvc := service.NewKlineService(provider, klineRepo)
	indicatorSvc := service.NewIndicatorService(klineSvc, indiRepo)
	updateSvc := service.NewUpdateService(provider, klineRepo, wmRepo, calRepo, secRepo, indicatorSvc)

	ctx := context.Background()
	var list []string
	if *codes != "" {
		list = utils.SplitCSV(*codes)
	} else {
		list, err = updateSvc.SyncSecuritiesList(ctx)
		if err != nil {
			slog.Error("sync securities", "err", err)
			os.Exit(1)
		}
	}
	slog.Info("backfill start", "count", len(list), "years", *years)
	_ = years
	for i, code := range list {
		if err := updateSvc.IncrementalOne(ctx, code, nil); err != nil {
			slog.Warn("kline", "code", code, "err", err)
			continue
		}
		if err := updateSvc.BackfillIndicators(ctx, code, nil); err != nil {
			slog.Warn("indicators", "code", code, "err", err)
		}
		if (i+1)%50 == 0 {
			slog.Info("progress", "done", i+1, "total", len(list))
		}
	}
	slog.Info("backfill done")
}
