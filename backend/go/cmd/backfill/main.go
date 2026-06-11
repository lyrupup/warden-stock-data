package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/warden-stock/warden-stock-data/internal/config"
	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/integration/quant"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/database"
	"github.com/warden-stock/warden-stock-data/pkg/utils"
)

func main() {
	flag.String("years", "5", "years of history (由 quant BACKFILL_START_DATE 控制)")
	codes := flag.String("codes", "", "comma-separated stock codes, empty = all from securities")
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
	quantClient := quant.NewClient(cfg.QuantBaseURL, cfg.InternalToken)
	klineRepo := repository.NewKlineRepository(db)
	wmRepo := repository.NewWatermarkRepository(db)
	calRepo := repository.NewCalendarRepository(db)
	secRepo := repository.NewSecurityRepository(db)
	updateSvc := service.NewUpdateService(provider, quantClient, klineRepo, wmRepo, calRepo, secRepo)

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
	slog.Info("backfill start", "count", len(list))
	for i, code := range list {
		if err := updateSvc.SyncKlineFull(ctx, code); err != nil {
			slog.Warn("kline full", "code", code, "err", err)
			continue
		}
		if (i+1)%50 == 0 {
			slog.Info("progress", "done", i+1, "total", len(list))
		}
	}
	slog.Info("backfill done")
}
