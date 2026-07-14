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
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
)

func redisOptions(cfg *config.Config) asynq.RedisClientOpt {
	return queueinfra.NewRedisOptions(cfg.Redis)
}

func Dispatcher(cfg *config.Config, _ *gorm.DB) (queue.Dispatcher, error) {
	return queueinfra.NewDispatcher(redisOptions(cfg)), nil
}

func Inspector(cfg *config.Config, _ *gorm.DB) (queue.Inspector, error) {
	return queueinfra.NewRedisInspector(redisOptions(cfg)), nil
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

	return queueinfra.NewRedisWorker(redisOptions(cfg), cfg.Queue, registry), nil
}
