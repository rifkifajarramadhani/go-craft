package bootstrap

import (
	"fmt"
	"log/slog"

	"__GO_CRAFT_MODULE__/internal/adapter/jobs"
	queueinfra "__GO_CRAFT_MODULE__/internal/adapter/queue"
	"__GO_CRAFT_MODULE__/internal/adapter/smtp"
	"__GO_CRAFT_MODULE__/internal/auth"
	"__GO_CRAFT_MODULE__/internal/config"
	"__GO_CRAFT_MODULE__/internal/queue"
	"gorm.io/gorm"
)

func Dispatcher(_ *config.Config, db *gorm.DB) (queue.Dispatcher, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required for database queue")
	}

	return queueinfra.NewDatabaseDispatcher(db), nil
}

func Inspector(cfg *config.Config, db *gorm.DB) (queue.Inspector, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required for database queue")
	}

	return queueinfra.NewDatabaseInspector(db, cfg.Queue.Queues), nil
}

func Worker(cfg *config.Config, db *gorm.DB, logger *slog.Logger) (queue.Worker, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required by worker job handlers")
	}

	repository := identityRepository(db)
	maintenance := auth.NewMaintenanceService(repository)

	mailTransport, err := smtp.NewTransport(cfg.Mail)
	if err != nil {
		return nil, err
	}

	registry := queue.NewHandlerRegistry()
	if err := jobs.RegisterHandlers(registry, maintenance, mailTransport, logger); err != nil {
		return nil, fmt.Errorf("register job handlers: %w", err)
	}

	return queueinfra.NewDatabaseWorker(db, cfg.Queue, registry, logger), nil
}
