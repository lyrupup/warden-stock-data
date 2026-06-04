package database

import (
	"fmt"
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type Options struct {
	Host, User, Password, DBName, SSLMode string
	Port                                    int
}

func Connect(opt Options) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		opt.Host, opt.Port, opt.User, opt.Password, opt.DBName, opt.SSLMode,
	)
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

func AutoMigrate(db *gorm.DB) {
	for _, m := range model.MigrateModels {
		if err := db.AutoMigrate(m); err != nil {
			slog.Warn("auto migrate skipped", "model", fmt.Sprintf("%T", m), "err", err)
		}
	}
}
