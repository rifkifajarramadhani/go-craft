// Package database exposes the selected persistence driver to process entrypoints.
package database

import (
	"context"
	"log/slog"

	sqliteadapter "__GO_CRAFT_MODULE__/internal/adapter/sqlite"
	"__GO_CRAFT_MODULE__/internal/config"
	"gorm.io/gorm"
)

func Open(ctx context.Context, cfg config.DatabaseConfig, logger *slog.Logger) (*gorm.DB, error) {
	return sqliteadapter.Open(ctx, cfg.Name, logger)
}

func Close(db *gorm.DB) error { return sqliteadapter.Close(db) }
