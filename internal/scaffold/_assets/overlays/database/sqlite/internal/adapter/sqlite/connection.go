package sqliteadapter

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func Open(ctx context.Context, path string, _ *slog.Logger) (*gorm.DB, error) {
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite directory: %w", err)
		}
	}

	values := url.Values{}
	values.Add("_pragma", "busy_timeout(10000)")
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "journal_mode(WAL)")
	values.Set("_txlock", "immediate")
	dsn := "file:" + path + "?" + values.Encode()

	db, err := gorm.Open(gormlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return db, nil
}

func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
