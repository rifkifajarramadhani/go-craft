// Package database exposes the selected persistence driver to process entrypoints.
package database

import (
	"context"
	"fmt"
	"log/slog"

	postgresadapter "__GO_CRAFT_MODULE__/internal/adapter/postgres"
	"__GO_CRAFT_MODULE__/internal/config"
	"gorm.io/gorm"
)

func Open(ctx context.Context, cfg config.DatabaseConfig, logger *slog.Logger) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name)

	return postgresadapter.Open(ctx, dsn, logger)
}

func Close(db *gorm.DB) error { return postgresadapter.Close(db) }
